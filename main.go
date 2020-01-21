package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	expect "github.com/Netflix/go-expect"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"strconv"
	"syscall"
)

type QemuConfig struct {
	Run             bool
	Path            string
	Drive           string
	Kernel          string
	Events          string
	Trace           string
	NumCores        string `yaml:"num_cores"`
	TraceMappingOut string `yaml:"trace_mapping_out"`
}

type CacheSimulatorConfig struct {
	Run         bool
	Path        string
	Distributed bool
	N           int
	Output      string
}

type Config struct {
	Benchmark        string
	BenchmarkSize    string `yaml:"benchmark_size"`
	// CR3filePath      string
	// PgTableDumpPath  string
	CacheSimulator   CacheSimulatorConfig
	Qemu             QemuConfig
	// CRValuesPath     string
	MemoryDumpPath string
	TracingFromStart bool `yaml:"tracing_from_start"`
}

func LoadConfig() *Config {
	var configPath *string
	configPath = flag.String("config", "", "path of the config file to use")
	flag.Parse()
	if configPath != nil && *configPath == "" {
		log.Fatal("Configpath not specified!\n")
	}
	log.Println("Using config at path:", *configPath)
	bytes, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal("Unable to read config file: ", err)
	}
	//Define default config values
	config := Config{
		// PgTableDumpPath: "pgtable-dump",
		// CR3filePath:     "cr3checker",
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
	// Print whole config to be able to see what config was used from the logs
	fmt.Printf("%+v\n", config)
	fmt.Println("Fromstart:", config.TracingFromStart)

	if config.Benchmark == "" {
		log.Fatalf("Benchmark not defined!")
	}
	return &config
}

func main() {
	var cacheSimCMD *exec.Cmd
	config := LoadConfig()
	//	if !config.Usepipe {
	// _, err := os.Stat(config.PgTableDumpPath)
	// if err != nil && os.IsNotExist(err) {
	// 	err = os.MkdirAll(config.PgTableDumpPath, 0700)
	// 	if err != nil {
	// 		log.Fatal("Unable to create dir for pgtable dump: ", err)
	// 	}
	// } else if err != nil {
	// 	log.Fatal("Unable to stat pgtable dump dir")
	// }



	// c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// if config.Qemu.Run {
	// 	if config.CacheSimulator.Run {
	// 		startCacheSimulator(cacheSimCMD, config)
	// 	}
	// 	defer func() {
	// 		if config.CacheSimulator.Run {
	// 			log.Println("Killing cachesim!\n")
	// 			if cacheSimCMD.Process != nil {
	// 				err = cacheSimCMD.Process.Signal(syscall.SIGINT)
	// 				if err != nil {
	// 					log.Println("Unable to kill cachesim:", err)
	// 				}
	// 			}
	// 		}
	// 	}()
	// 	time.Sleep(3*time.Second)
	// 	startQemu(c, &config.Qemu)
	// 	defer func() {
	// 		if config.Qemu.Run {
	// 			log.Println("Killing qemu!\n")
	// 			qemuKillCMD := exec.Command("./killqemu.sh")
	// 			runCMD(qemuKillCMD, true)
	// 		}
	// 	}()
	// 	log.Println("Sleeping!")
	// 	time.Sleep(5 * time.Second)

	// 	log.Println("Waiting for login")
	// 	c.ExpectString("localhost login:")
	// 	c.SendLine("root")
	// 	c.ExpectString("Password:")
	// 	c.SendLine("")
	// 	log.Println("Successfully logged in")
	// 	c.ExpectString("# ")
	// 	c.SendLine("cd parsec-3.0/")
	// 	c.ExpectString("# ")
	// 	c.SendLine("source env.sh")
	// 	c.ExpectString("# ")
	// 	time.Sleep(2 * time.Second)
	// 	if config.CacheSimulator.Run && !config.TracingFromStart {
	// 		log.Println("Pausing QEMU to dump the pagetables the benchmark")
	// 		runMonitorCMD("stop")
	// 		runMonitorCMD("flush-simple-trace-file")
	// 		time.Sleep(3 * time.Second) // Make sure all cr3 change events are received and written by input reader
	// 		for _, cr3 := range readCR3s(config.CR3filePath) {
	// 			if cr3 != "" {
	// 				runMonitorCMD("dump-pagetable " + config.PgTableDumpPath + "/" + cr3 + " " + cr3)
	// 			}
	// 		}
	// 		readCRValues(config.CRValuesPath)
	// 	}
	// 	log.Println("Enabling tracing")
	// 	runCMD(exec.Command("./enabletracing.sh"), true)
	// 	if config.CacheSimulator.Run && !config.TracingFromStart {
	// 		runMonitorCMD("cont")
	// 	}

	// 	log.Printf("Enabled tracing and resumed simulator!\n")
	// 	time.Sleep(1 * time.Second)

	// 	startTime := time.Now()

	// 	c.SendLine("parsecmgmt -a run -p " + config.Benchmark + " -i " + config.BenchmarkSize + " -n " + config.Qemu.NumCores + " -s \"echo 'Starting' && time\"")
	// 	c.ExpectString("# ")
	// 	log.Println("Time spend on benchmark: ", time.Now().Sub(startTime))
	// }
}

func startQemu(c *expect.Console, config *QemuConfig) *exec.Cmd {
	log.Println("Using trace pipe:", config.Trace)
	log.Printf("Starting qemu with %s cores\n", config.NumCores)
	log.Println("./runqemu.sh", config.Path, config.Drive, config.Kernel, config.NumCores, config.Events, config.Trace)
	cmd := exec.Command("./runqemu.sh", config.Path, config.Drive, config.Kernel, config.NumCores, config.Events, config.Trace)
	// qemuOptions := []string{
	// 	"-drive", "file=" + config.Drive + ",format=qcow2",
	// 	"-m", "12G", "-nographic",
	// 	"-net", "nic", "-net", "user,hostfwd=tcp::2222-:22",
	// 	"-append", `"root=/dev/sda3 console=ttyS0 zswap.enabled=1 cgroup_enable=memory swapaccount=1 storage-driver=overlay2"`,
	// 	"-no-reboot",
	// 	"-kernel", config.Kernel,
	// 	"-monitor", "unix:/tmp/qemu-monitor,server,nowait",
	// 	"-smp", "1",
	// 	"-trace", "events=" + config.Events + ",file=" + config.Trace}
	// cmd := exec.Command(config.Path, qemuOptions...)
	fmt.Println("Running:", strings.Join(cmd.Args, " "))

	runCMDWithExec(cmd, c, false)
	fmt.Println("Started qemu!\n")
	return cmd
}

func startCacheSimulator(cacheSimCMD *exec.Cmd, config *Config) {
	log.Println("Starting cache simulator!")
	if config.CacheSimulator.Distributed && config.TracingFromStart {
		cacheSimCMD = exec.Command("mpirun", "-v", "-np", strconv.Itoa(config.CacheSimulator.N), config.CacheSimulator.Path, "n", "n", config.Qemu.Trace)
	} else if config.CacheSimulator.Distributed {
		cacheSimCMD = exec.Command("mpirun", "-v", "-np", strconv.Itoa(config.CacheSimulator.N), config.CacheSimulator.Path, "y", "n", config.Qemu.Trace, config.PgTableDumpPath, config.CR3filePath, config.CRValuesPath, config.PgTableDumpPath)
	} else {
		// gcc  -DSIMULATE_CACHE=1 -DSIMULATE_MEMORY=1 -DSIMULATE_ADDRESS_TRANSLATION=1  -I. simulator/simple/simulator.c pagetable/pagetable.c cache/*.c mappingreader/mappingreader.c pipereader/pipereader.c memory/memory.c control_registe
		//	rs/control_registers.c -o bin/main && time bin/main mappingreader/trace_mapping pipe   /media/ssd/out
		cacheSimCMD = exec.Command(config.CacheSimulator.Path, config.Qemu.TraceMappingOut, config.Qemu.Trace, config.CacheSimulator.Output, config.CR3filePath)
	}
	fmt.Println("Starting cache simulator:", strings.Join(cacheSimCMD.Args, " "))
	runCMD(cacheSimCMD, false)
	log.Println("Started cache simulator!")
}

func runCMDWithExec(cmd *exec.Cmd, c *expect.Console, wait bool) {
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if wait {
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal("Unable to exec and wait for command!", err)
		}
		log.Println(string(out))
	} else {
		err := cmd.Start()
		if err != nil {
			log.Fatal("Unable to execute command:", err)
		}
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

func readCRValues(path string) {
	log.Println("Finding initial cr values!")

	out, err := exec.Command("./monitorcommand.sh", "info registers -a").Output()
	if err != nil {
		log.Fatal(err)
	}

	cpuRegex := regexp.MustCompile(`CPU#\d+-0x([0-9a-f]+)`)
	crRegex := regexp.MustCompile(`CR\d=([0-9a-f]+)\s`)
	cpus := cpuRegex.FindAllStringSubmatch(string(out), -1)
	crs := crRegex.FindAllStringSubmatch(string(out), -1)
	//	fmt.Printf("%#+v\n", crs)
	crIndex := 0
	f, err := os.OpenFile(path, os.O_WRONLY, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	//fmt.Printf("%#+v\n", cpus)
	for i := range cpus {
		_, err = f.Write([]byte(cpus[i][1] + "\n"))
		if err != nil {
			log.Fatal(err)
		}
		//fmt.Println(cpus[i][1])
		for j := 0; j <= 4; j++ {
			if j == 1 {
				_, err = f.Write([]byte("0\n"))
				if err != nil {
					log.Fatal(err)
				}
				continue
			}
			//fmt.Println(crs[crIndex][1])
			_, err = f.Write([]byte(crs[crIndex][1] + "\n"))
			if err != nil {
				log.Fatal(err)
			}
			crIndex++
		}
	}
	f.Sync()
}

func readCR3s(inputPath string) []string {
	log.Println("Reading cr3 values at:", inputPath)
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
	if err != io.EOF {
		fmt.Println("Error reading cr3 values:", err)
	}
	ret := []string{}
	for _, value := range cr3s {
		ret = append(ret, value)
	}
	log.Println(ret)
	return ret
}
