package pcap

import (
	"0E7/service/config"
	"0E7/service/database"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/gopacket"
	"github.com/google/gopacket/ip4defrag"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/reassembly"
	"github.com/google/uuid"
)

var (
	decoder      = ""
	lazy         = false
	checksum     = false
	nohttp       = true
	snaplen      = 65536
	tstype       = ""
	promisc      = true
	flag_regex   = ""
	bpf          = ""
	nonstrict    = false
	experimental = false
	flushAfter   = ""
)

type FlowItem struct {
	From string `json:"f"`
	Data string `json:"d"`
	B64  string `json:"b"`
	Time int    `json:"t"`
}

type FlowEntry struct {
	SrcPort      int
	DstPort      int
	SrcIp        string
	DstIp        string
	Time         int
	Duration     int
	NumPackets   int
	Blocked      bool
	Filename     string
	Fingerprints []uint32
	Suricata     []int
	Flow         []FlowItem
	Tags         []string
	Size         int
}

func reassemblyCallback(entry FlowEntry) {
	ParseHttpFlow(&entry)
	if flag_regex != "" {
		ApplyFlagTags(&entry, flag_regex)
	}
	for idx := 0; idx < len(entry.Flow); idx++ {
		flowItem := &entry.Flow[idx]
		flowItem.B64 = base64.StdEncoding.EncodeToString([]byte(flowItem.Data))
		flowItem.Data = strings.Map(func(r rune) rune {
			if r < 128 {
				return r
			}
			return -1
		}, flowItem.Data)
	}
	Fingerprints, err := json.Marshal(entry.Fingerprints)
	if err != nil {
		log.Println("Fingerprints Error:", err)
		return
	}
	Suricata, err := json.Marshal(entry.Suricata)
	if err != nil {
		log.Println("Suricata Error:", err)
		return
	}
	Flow, err := json.Marshal(entry.Flow)
	if err != nil {
		log.Println("Flow Error:", err)
		return
	}
	// 使用gzip压缩保存flow数据，便于gin中间件直接使用
	flowFile := filepath.Join("flow", uuid.New().String()+".gz")

	// 创建gzip压缩文件
	file, err := os.Create(flowFile)
	if err != nil {
		log.Println("Create flow file failed:", err)
		return
	}
	defer file.Close()

	// 创建gzip writer
	gzWriter := gzip.NewWriter(file)

	// 写入压缩数据
	_, err = gzWriter.Write([]byte(Flow))
	if err != nil {
		log.Println("Write compressed flow data failed:", err)
		gzWriter.Close()
		return
	}

	// 关闭gzip writer，这会写入gzip尾部
	err = gzWriter.Close()
	if err != nil {
		log.Println("Close gzip writer failed:", err)
		return
	}

	Tags, err := json.Marshal(entry.Tags)
	if err != nil {
		log.Println("Tags Error:", err)
		return
	}

	pcapRecord := database.Pcap{
		SrcPort:      fmt.Sprintf("%d", entry.SrcPort),
		DstPort:      fmt.Sprintf("%d", entry.DstPort),
		SrcIP:        entry.SrcIp,
		DstIP:        entry.DstIp,
		Time:         entry.Time,
		Duration:     entry.Duration,
		NumPackets:   entry.NumPackets,
		Blocked:      fmt.Sprintf("%t", entry.Blocked),
		Filename:     entry.Filename,
		Fingerprints: string(Fingerprints),
		Suricata:     string(Suricata),
		Flow:         flowFile,
		Tags:         string(Tags),
		Size:         fmt.Sprintf("%d", entry.Size),
	}
	err = config.Db.Create(&pcapRecord).Error

	if err != nil {
		log.Println("Insert Error:", err)
	}
}

func Setbpf(str string) {
	bpf = str
}
func SetFlagRegex(flag string) {
	flag_regex = flag
}
func ParsePcapfile(fname string) {
	handlePcapUri(fname, bpf)
}
func WatchDir(watch_dir string) {
	flowDir := "flow/"
	if _, err := os.Stat(flowDir); os.IsNotExist(err) {
		err := os.MkdirAll(flowDir, os.ModePerm)
		if err != nil {
			log.Println("Create flow dir failed:", err)
			return
		}
	}

	stat, err := os.Stat(watch_dir)
	if err != nil {
		log.Println("Failed to open the watch_dir with error: ", err)
		err := os.MkdirAll(watch_dir, os.ModePerm)
		if err != nil {
			log.Println("Create dir failed:", err)
			return
		}
		stat, err = os.Stat(watch_dir)
		if err != nil {
			log.Println("Stat dir failed:", err)
			return
		}
	}

	if !stat.IsDir() {
		log.Println("watch_dir is not a directory")
		return
	}

	log.Println("Monitoring dir: ", watch_dir)

	files, err := ioutil.ReadDir(watch_dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".pcap") || strings.HasSuffix(file.Name(), ".pcapng") {
			handlePcapUri(filepath.Join(watch_dir, file.Name()), bpf)
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Rename|fsnotify.Create) != 0 {
					if strings.HasSuffix(event.Name, ".pcap") || strings.HasSuffix(event.Name, ".pcapng") {
						log.Println("Found new file", event.Name, event.Op.String())
						handlePcapUri(event.Name, bpf)
						time.Sleep(1 * time.Second)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("watcher error:", err)
			}
		}
	}()

	err = watcher.Add(watch_dir)
	if err != nil {
		log.Fatal(err)
	}

	select {}
}

/*
	func main() {
		defer util.Run()()

		flag.Parse()
		if flag.NArg() < 1 && *watch_dir == "" {
			log.Fatal("Usage: ./go-importer <file0.pcap> ... <fileN.pcap>")
		}

		// If no flag regex was supplied via cli, check the env
		if *flag_regex == "" {
			*flag_regex = os.Getenv("FLAG_REGEX")
			// if that didn't work, warn the user and continue
			if *flag_regex == "" {
				log.Print("WARNING; no flag regex found. No flag-in or flag-out tags will be applied.")
			}
		}

		if *pcap_over_ip == "" {
			*pcap_over_ip = os.Getenv("PCAP_OVER_IP")
		}

		if *bpf == "" {
			*bpf = os.Getenv("BPF")
		}

		if *pcap_over_ip != "" {
			log.Println("Connecting to PCAP-over-IP:", *pcap_over_ip)
			tcpServer, err := net.ResolveTCPAddr("tcp", *pcap_over_ip)
			if err != nil {
				log.Fatal(err)
			}
			conn, err := net.DialTCP("tcp", nil, tcpServer)
			if err != nil {
				log.Fatal(err)
			}
			defer conn.Close()
			pcapFile, err := conn.File()
			if err != nil {
				log.Fatal(err)
			}
			defer pcapFile.Close()
			handlePcapFile(pcapFile, *pcap_over_ip, *bpf)
		} else {
			// Pass positional arguments to the pcap handler
			for _, uri := range flag.Args() {
				handlePcapUri(uri, *bpf)
			}
			if *watch_dir != "" {
				watchDir(*watch_dir)
			}
		}
	}
*/
func checkfile(fname string, status bool) bool {
	var count int64
	err := config.Db.Model(&database.PcapFile{}).Where("filename = ?", fname).Count(&count).Error
	if err != nil {
		log.Println("Failed to query database:", err)
		return false
	}
	if count == 0 {
		if status {
			pcapFile := database.PcapFile{
				Filename: fname,
			}
			err = config.Db.Create(&pcapFile).Error
			if err != nil {
				log.Println("Failed to insert pcap file:", err)
			}
		}
		return true
	} else {
		return false
	}
}
func handlePcapUri(fname string, bpf string) {
	var handle *pcap.Handle
	var err error

	if handle, err = pcap.OpenOffline(fname); err != nil {
		log.Println("PCAP OpenOffline error:", err)
		return
	}
	defer handle.Close()

	if bpf != "" {
		if err := handle.SetBPFFilter(bpf); err != nil {
			log.Println("Set BPF Filter error: ", err)
			return
		}
	}

	processPcapHandle(handle, fname)
}

func processPcapHandle(handle *pcap.Handle, fname string) {
	if !checkfile(fname, false) {
		return
	}
	var source *gopacket.PacketSource
	nodefrag := false
	linktype := handle.LinkType()
	switch linktype {
	case layers.LinkTypeIPv4:
		source = gopacket.NewPacketSource(handle, layers.LayerTypeIPv4)
		break
	default:
		source = gopacket.NewPacketSource(handle, linktype)
	}

	source.Lazy = lazy
	source.NoCopy = true
	count := 0
	bytes := int64(0)
	defragger := ip4defrag.NewIPv4Defragmenter()

	streamFactory := &tcpStreamFactory{source: fname, reassemblyCallback: reassemblyCallback}
	streamPool := reassembly.NewStreamPool(streamFactory)
	assembler := reassembly.NewAssembler(streamPool)

	var nextFlush time.Time
	var flushDuration time.Duration
	var err error
	if flushAfter != "" {
		flushDuration, err = time.ParseDuration(flushAfter)
		if err != nil {
			log.Fatal("invalid flush duration: ", flushAfter)
		}
		nextFlush = time.Now().Add(flushDuration / 2)
		log.Println("Starting PCAP loop!")
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	for packet := range source.Packets() {
		count++
		data := packet.Data()
		bytes += int64(len(data))
		done := false

		if !nextFlush.IsZero() {
			// Check to see if we should flush the streams we have that haven't seen any new data in a while.
			// Note that pcapOpenOfflineFile is blocking so we need at least see some packets passing by to get here.
			if time.Since(nextFlush) > 0 {
				log.Printf("flushing all streams that haven't seen packets in the last %s", flushAfter)
				assembler.FlushCloseOlderThan(time.Now().Add(-flushDuration))
				nextFlush = time.Now().Add(flushDuration / 2)
			}
		}

		// defrag the IPv4 packet if required
		// (UNTODO; IPv6 will not be defragged)
		ip4Layer := packet.Layer(layers.LayerTypeIPv4)
		if !nodefrag && ip4Layer != nil {
			ip4 := ip4Layer.(*layers.IPv4)
			l := ip4.Length
			newip4, err := defragger.DefragIPv4(ip4)
			if err != nil {
				log.Fatalln("Error while de-fragmenting", err)
			} else if newip4 == nil {
				continue // packet fragment, we don't have whole packet yet.
			}
			if newip4.Length != l {
				pb, ok := packet.(gopacket.PacketBuilder)
				if !ok {
					panic("Not a PacketBuilder")
				}
				nextDecoder := newip4.NextLayerType()
				nextDecoder.Decode(newip4.Payload, pb)
			}
		}

		transport := packet.TransportLayer()
		if transport == nil {
			continue
		}

		switch transport.LayerType() {
		case layers.LayerTypeTCP:
			tcp := transport.(*layers.TCP)
			c := Context{
				CaptureInfo: packet.Metadata().CaptureInfo,
			}
			assembler.AssembleWithContext(packet.NetworkLayer().NetworkFlow(), tcp, &c)
			break
		default:
			// pass
		}

		select {
		case <-signalChan:
			fmt.Fprintf(os.Stderr, "\nCaught SIGINT: aborting\n")
			done = true
		default:
			// NOP: continue
		}
		if done {
			break
		}
	}

	assembler.FlushAll()
	streamFactory.WaitGoRoutines()
	checkfile(fname, true)
}
