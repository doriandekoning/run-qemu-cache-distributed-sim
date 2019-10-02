package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	expect "github.com/Netflix/go-expect"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

type QemuConfig struct {
	Run    bool
	Path   string
	Drive  string
	Kernel string
	Events string
	Trace  string
}

type CacheSimulatorConfig struct {
	Run  bool
	Path string
	N    int
}

type Config struct {
	Usepipe         bool
	Benchmark       string
	CR3filePath     string
	PgTableDumpPath string
	CacheSimulator  CacheSimulatorConfig
	Qemu            QemuConfig
}

func main() {
	var configPath *string
	var cacheSimCMD *exec.Cmd
	configPath = flag.String("config", "config.yaml", "path of the config file to use")
	flag.Parse()
	log.Println("Using config at path:", *configPath)
	bytes, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal("Unable to read config file: ", err)
	}
	//Define default config values
	config := Config{
		PgTableDumpPath: "pgtable-dump",
		CR3filePath:     "cr3checker",
		CacheSimulator: CacheSimulatorConfig{
			Path: "cache-simulator",
			N:    2,
		},
		Qemu: QemuConfig{
			Trace: "trace",
		},
	}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		log.Fatalf("Unable to parse config file: ", err)
	}
	if config.Benchmark == "" {
		log.Fatalf("Benchmark not defined!")
	}

	//	if !config.Usepipe {
	_, err = os.Stat(config.PgTableDumpPath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(config.PgTableDumpPath, 0700)
		if err != nil {
			log.Fatal("Unable to create dir for pgtable dump: ", err)
		}
	} else if err != nil {
		log.Fatal("Unable to stat pgtable dump dir")
	}
	//}

	//TODO make usepipe support

	c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	if err != nil {
		log.Fatal(err)
	}
	if config.Qemu.Run {
		qemuCMD := startQemu(c, &config.Qemu)
		defer func() {
			if config.Qemu.Run {
				log.Println("Killing qemu!\n")
				err := qemuCMD.Process.Signal(syscall.SIGKILL)
				if err != nil {
					log.Println("Unable to kill qemu:", err)
				}
			}
			if config.CacheSimulator.Run && config.Usepipe {
				log.Println("Killing cachesim!\n")
				err = cacheSimCMD.Process.Signal(syscall.SIGINT)
				if err != nil {
					log.Println("Unable to kill cachesim:", err)
				}
			}
			c.Close()
		}()
		time.Sleep(1 * time.Second)
		if config.CacheSimulator.Run && config.Usepipe {
			log.Println("Starting cache simulator!")
			cacheSimCMD = exec.Command("mpirun", "-v", "-np", strconv.Itoa(config.CacheSimulator.N), config.CacheSimulator.Path, config.PgTableDumpPath, config.Qemu.Trace, "y", config.CR3filePath)
			runCMD(cacheSimCMD, false)
		}
		c.ExpectString("localhost login:")
		c.SendLine("root")
		c.ExpectString("Password:")
		c.SendLine("")
		c.ExpectString("# ")
		c.SendLine("cd parsec-3.0/")
		c.ExpectString("# ")
		c.SendLine("source env.sh")
		c.ExpectString("# ")
		runMonitorCMD("stop")
		for _, cr3 := range readCR3s(config.CR3filePath) {
			if cr3 != "" {
				runMonitorCMD("dump-pagetable " + config.PgTableDumpPath + "/" + cr3 + " " + cr3)
			}
		}

		runCMD(exec.Command("./enabletracing.sh"), true)
		runMonitorCMD("cont")
		log.Printf("Enabled tracing and resumed simulator!\n")
		time.Sleep(1 * time.Second)
		startTime := time.Now()
		c.SendLine("parsecmgmt -a run -p " + config.Benchmark + " -i simlarge -n 8 -s \"echo 'Starting' && time\"")
		c.ExpectString("# ")
		log.Println("Time spend on benchmark: ", time.Now().Sub(startTime))
	}
	//Start cache simulator
	if config.CacheSimulator.Run && !config.Usepipe {
		runCMD(exec.Command("mpirun", "-v", "-np", strconv.Itoa(config.CacheSimulator.N), config.CacheSimulator.Path, config.PgTableDumpPath, config.Qemu.Trace, "n", config.CR3filePath), true)
	}
}

func startQemu(c *expect.Console, config *QemuConfig) *exec.Cmd {
	log.Println("Using trace pipe:", config.Trace)
	cmd := exec.Command("./runqemu.sh", config.Path, config.Drive, config.Kernel, config.Events, config.Trace)
	runCMDWithExec(cmd, c, false)
	return cmd
}

func runCMDWithExec(cmd *exec.Cmd, c *expect.Console, wait bool) {
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if !wait {
		err := cmd.Start()
		if err != nil {
			log.Fatal("Unable to execute command:", err)
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal("Unable to exec and wait for command!", err)
		}
		log.Println(string(out))
	}

}

func runCMD(cmd *exec.Cmd, wait bool) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Fatal("Unable to start command:", err)
	}
	if wait {
		err = cmd.Wait()
		if err != nil {
			log.Fatal("Unable to wait for command:", err)
		}
	}
}

func runMonitorCMD(command string) {
	runCMD(exec.Command("./monitorcommand.sh", command), true)

}

func readCR3s(inputPath string) []string {
	file, err := os.Open(inputPath)
	if err != nil {
		log.Fatal(err)
	}
	cr3s := map[uint64]string{}
	var cr3 uint64
	for err == nil {
		err = binary.Read(file, binary.LittleEndian, &cr3)
		cr3s[cr3] = strconv.FormatUint(cr3, 10)
	}
	fmt.Println("Error reading cr3 values:", err)
	ret := []string{}
	for _, value := range cr3s {
		ret = append(ret, value)
	}
	return ret
}
