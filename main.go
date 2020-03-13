package main

import (
	"flag"
	"fmt"

	expect "github.com/Netflix/go-expect"
	semaphore "github.com/dangerousHobo/go-semaphore"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type QemuConfig struct {
	Run                    bool
	Path                   string
	Drive                  string
	Kernel                 string
	Events                 string
	Trace                  string
	NumCores               string `yaml:"num_cores"`
	TraceEventIDMappingOut string `yaml:"trace_event_id_mapping"`
	QmpTelnetPort          int    `yaml:"qmp_telnet_port"`
	MemorySize             string `yaml:"memory_size"`
}

type CacheSimulatorConfig struct {
	Run         bool
	Path        string
	Distributed bool
	N           int
	Output      string
}

type Config struct {
	Benchmark     string
	BenchmarkSize string `yaml:"benchmark_size"`
	// CR3filePath      string
	// PgTableDumpPath  string
	CacheSimulator   CacheSimulatorConfig
	Qemu             QemuConfig
	CRValuesPath     string `yaml:"cr_values_path"`
	MemoryDumpPath   string `yaml:"memory_dump_path"`
	TracingFromStart bool   `yaml:"tracing_from_start"`
	MemrangePath     string `yaml:"mem_range_path"`
	//Benchmark        string `yaml:"benchmark"`
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
	if config.CRValuesPath == "" {
		log.Fatalf("Crvalues path not set at cr_values_path")
	}
	return &config
}

func main() {
	var cacheSimCMD exec.Cmd
	config := LoadConfig()
	var start_simulation_semaphore = &semaphore.Semaphore{}
	err := start_simulation_semaphore.Open("/memtracing_start_sem", syscall.S_IRWXU, 0)
	if err != nil {
		log.Panic("Unable to open semaphore")
	}

	c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	if err != nil {
		log.Fatal(err)
	}
	if config.Qemu.Run {
		startQemu(c, &config.Qemu)
		defer func() {
			if config.Qemu.Run {
				log.Println("Killing qemu!\n")
				qemuKillCMD := exec.Command("./killqemu.sh")
				runCMD(qemuKillCMD, true)
			}
		}()
	}

	if config.CacheSimulator.Run {
		startCacheSimulator(&cacheSimCMD, config)
	}
	defer func() {
		if config.CacheSimulator.Run {
			log.Println("Killing cachesim!\n")
			if cacheSimCMD.Process != nil {
				err := cacheSimCMD.Process.Signal(syscall.SIGINT)
				if err != nil {
					log.Println("Unable to kill cachesim:", err)
				}
			}
		}
	}()

	log.Println("Sleeping!")
	time.Sleep(1 * time.Second)

	qmpConn := InitQMPConnection("/tmp/qemu-qmp")
	ranges := qmpConn.InfoE820()
	fmt.Printf("outputting memrange to: %s\n", config.MemrangePath)
	f, err := os.Create(config.MemrangePath)
	if err != nil {
		log.Fatal(err)
	}
	str := fmt.Sprintf("%x\n", len(ranges))
	n, err := f.Write([]byte(str))
	if err != nil || n != len([]byte(str)) {
		f.Close()
		log.Fatal("Cannot write to memrange file!:", err)
	}
	for _, r := range ranges {
		fmt.Printf("Range, start:%x, end: %x\n", r.Start, r.End)
		str = fmt.Sprintf("%x-%x\n", r.Start, r.End)
		n, err = f.Write([]byte(str))
		if err != nil || n != len([]byte(str)) {
			f.Close()
			log.Fatal("Cannot write to memrange file!:", err)
		}
	}
	f.Close()

	time.Sleep(3 * time.Second)

	log.Println("Waiting for login")
	//c.ExpectString("localhost login:")
	//c.SendLine("root")
	c.ExpectString("ubuntu login:")

	c.SendLine("ubuntu")
	c.ExpectString("Password:")
	//c.SendLine("")
	c.SendLine("asdfqwer")
	log.Println("Successfully logged in")
	//	c.ExpectString("$ ")
	c.ExpectString("$ ")
	c.SendLine("cd parsec-3.0/")
	c.ExpectString("$ ")
	c.SendLine("source env.sh")
	/*		c.SendLine("cat /proc/meminfo | head -n 1")
			c.ExpectString("$ ")*/
	//	c.SendLine("tar -xvf parsec-3.0/pkgs/apps/freqmine/inputs/input_simsmall.tar")
	//	c.ExpectString("$ ")

	time.Sleep(2 * time.Second)
	c.ExpectString("$ ")
	fmt.Println("Stopping")
	qmpConn.Stop()
	fmt.Println("Sleeping for some time!\n")
	time.Sleep(2 * time.Second)
	readCRValues(config.CRValuesPath)
	for _, r := range ranges {
		qmpConn.pmemsave(r.Start, r.End, fmt.Sprintf("%s-%x", config.MemoryDumpPath, r.Start))
	}
	//	time.Sleep(10 * time.Second) // Give pmemsave a bit of time to complete
	qmpConn.SetTraceEvent("guest_update_cr", true)
	qmpConn.SetTraceEvent("guest_mem_load_before_exec", true)
	qmpConn.SetTraceEvent("guest_mem_store_before_exec", true)
	qmpConn.SetTraceEvent("guest_start_exec_tb", true)
	//	qmpConn.SetTraceEvent("guest_mem_before_exec", true)

	start_simulation_semaphore.Post()
	time.Sleep(20 * time.Second)
	fmt.Println("Continueing")
	fmt.Println("Started timing!\n")
	qmpConn.Cont()
	// if config.CacheSimulator.Run {
	// 	go func() {
	// 		cacheSimCMD.Wait()
	// 		log.Println("Cachesim exitted after: ", time.Now().Sub(startTime))

	// 	}()
	// }
	fmt.Println("Starting benchmark!\n")
	//	c.SendLine("parsecmgmt -a run -p " + config.Benchmark + " -i " + config.BenchmarkSize + " -n " + config.Qemu.NumCores + " -s \"echo 'Starting' && time\"")
	c.SendLine("cd ~")
	c.ExpectString("$ ")
	//c.SendLine("./parsec-2.1/pkgs/kernels/streamcluster/inst/amd64-linux.gcc.pre/bin/streamcluster 10 20 128 16384 16384 1000 none output.txt 1") //Large
	//	c.SendLine("./parsec-2.1/pkgs/kernels/streamcluster/inst/amd64-linux.gcc.pre/bin/streamcluster 10 20 32 4096 4069 1000 none output.txt 1") //Medium
	//	c.SendLine("./parsec-2.1/pkgs/kernels/streamcluster/inst/amd64-linux.gcc.pre/bin/streamcluster 10 20 32 4096 4096 1000 none output.txt 1") // Small
	//	c.SendLine("./parsec-2.1/pkgs/apps/blackscholes/inst/amd64-linux.gcc.pre/bin/blackscholes  1 in_16K.txt prices.txt") // Blackscholes medium
	log.Println("Running benchmark:", config.Benchmark)
	startTime := time.Now()
	c.SendLine(config.Benchmark)
	time.Sleep(1 * time.Second)
	c.ExpectString("$ ")
	fmt.Println("Time used by benchmark: ", time.Now().Sub(startTime))

	//	qmpConn.SetTraceEvent("guest_update_cr", false)
	//	qmpConn.SetTraceEvent("guest_mem_load_before_exec", false)
	//	qmpConn.SetTraceEvent("guest_mem_store_before_exec", false)
	//	qmpConn.SetTraceEvent("guest_start_exec_tb", false)
}

func startQemu(c *expect.Console, config *QemuConfig) *exec.Cmd {
	log.Println("Using trace pipe:", config.Trace)
	log.Printf("Starting qemu with %s cores\n", config.NumCores)
	log.Println("./runqemu.sh", config.Path, config.Drive, config.Kernel, config.NumCores, config.Events, config.Trace, config.MemorySize)
	cmd := exec.Command("./runqemu.sh", config.Path, config.Drive, config.Kernel, config.NumCores, config.Events, config.Trace, config.MemorySize)
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
		*cacheSimCMD = *exec.Command("mpirun", "-v", "-np", strconv.Itoa(config.CacheSimulator.N), config.CacheSimulator.Path, "n", "n", config.Qemu.Trace)
	} else if config.CacheSimulator.Distributed {
		cacheSimCMD = exec.Command("mpirun", "-v", "-np", strconv.Itoa(config.CacheSimulator.N), config.CacheSimulator.Path, config.Qemu.TraceEventIDMappingOut, "crvalues", config.Qemu.Trace, config.MemrangePath, "2", config.CacheSimulator.Output, config.MemoryDumpPath)
	} else {
		*cacheSimCMD = *exec.Command(config.CacheSimulator.Path, config.Qemu.TraceEventIDMappingOut, config.Qemu.Trace, config.CacheSimulator.Output, config.Qemu.NumCores, config.MemoryDumpPath, config.MemrangePath, config.CRValuesPath) //TODO add mem dump path
	}
	runCMD(cacheSimCMD, false)
	log.Println("Started cache simulator!", cacheSimCMD)
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

	fmt.Println(string(out))
	cpuRegex := regexp.MustCompile(`CPU#(\d+)`)
	crRegex := regexp.MustCompile(`CR\d=([0-9a-f]+)\s`)
	cpus := cpuRegex.FindAllStringSubmatch(string(out), -1)
	crs := crRegex.FindAllStringSubmatch(string(out), -1)
	fmt.Printf("%#+v\n", crs)
	crIndex := 0
	fmt.Println("Writing cr values to:", path)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fmt.Println(cpus)
	for i := range cpus {
		_, err = f.Write([]byte(cpus[i][1] + "\n"))
		if err != nil {
			log.Fatal(err)
		}
		for j := 0; j <= 4; j++ {
			if j == 1 {
				_, err = f.Write([]byte("0\n"))
				if err != nil {
					log.Fatal(err)
				}
				continue
			}
			_, err = f.Write([]byte(crs[crIndex][1] + "\n"))
			if err != nil {
				log.Fatal(err)
			}
			crIndex++
		}
	}
	f.Sync()
}
