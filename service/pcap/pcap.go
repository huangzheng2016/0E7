package pcap

import (
	"0E7/service/config"
	"0E7/service/database"
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
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
	"github.com/google/gopacket/pcapgo"
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

// calculateFileMD5 计算文件的MD5值
func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

type FlowItem struct {
	From string `json:"f"`
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

// SaveFlowAsPcap 将TCP流数据保存为pcap格式文件
func SaveFlowAsPcap(entry FlowEntry) string {
	flowUUID := uuid.New().String()

	// 根据压缩设置确定文件扩展名
	var pcapFile string
	if config.Server_pcap_zip {
		pcapFile = filepath.Join("flow", flowUUID+".pcap.gz")
	} else {
		pcapFile = filepath.Join("flow", flowUUID+".pcap")
	}

	// 创建pcap文件
	file, err := os.Create(pcapFile)
	if err != nil {
		log.Println("Create pcap file failed:", err)
		return ""
	}
	defer file.Close()

	var writer *pcapgo.Writer
	if config.Server_pcap_zip {
		// 创建gzip writer
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = pcapgo.NewWriter(gzWriter)
	} else {
		writer = pcapgo.NewWriter(file)
	}

	// 写入pcap文件头
	err = writer.WriteFileHeader(65536, layers.LinkTypeEthernet)
	if err != nil {
		log.Println("Write pcap file header failed:", err)
		return ""
	}

	// 解析源IP和目标IP
	srcIP := net.ParseIP(entry.SrcIp)
	dstIP := net.ParseIP(entry.DstIp)
	if srcIP == nil || dstIP == nil {
		log.Println("Invalid IP address:", entry.SrcIp, entry.DstIp)
		return ""
	}

	// 为每个FlowItem创建数据包
	for _, flowItem := range entry.Flow {
		// 创建以太网层
		ethernet := &layers.Ethernet{
			SrcMAC:       net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
			DstMAC:       net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
			EthernetType: layers.EthernetTypeIPv4,
		}

		// 创建IP层
		ip := &layers.IPv4{
			Version:  4,
			IHL:      5,
			TTL:      64,
			Protocol: layers.IPProtocolTCP,
		}

		// 创建TCP层
		tcp := &layers.TCP{
			Window: 65535,
		}

		// 根据数据方向设置源和目标
		if flowItem.From == "c" {
			// 客户端到服务器
			ip.SrcIP = srcIP
			ip.DstIP = dstIP
			tcp.SrcPort = layers.TCPPort(entry.SrcPort)
			tcp.DstPort = layers.TCPPort(entry.DstPort)
		} else {
			// 服务器到客户端
			ip.SrcIP = dstIP
			ip.DstIP = srcIP
			tcp.SrcPort = layers.TCPPort(entry.DstPort)
			tcp.DstPort = layers.TCPPort(entry.SrcPort)
		}

		// 设置TCP数据 - 从B64解码
		data, err := base64.StdEncoding.DecodeString(flowItem.B64)
		if err != nil {
			log.Println("Failed to decode B64 data:", err)
			continue
		}
		tcp.Payload = data

		// 计算校验和
		tcp.SetNetworkLayerForChecksum(ip)

		// 创建数据包缓冲区
		buf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		// 序列化数据包
		err = gopacket.SerializeLayers(buf, opts,
			ethernet,
			ip,
			tcp,
			gopacket.Payload(tcp.Payload),
		)
		if err != nil {
			log.Println("Serialize packet failed:", err)
			continue
		}

		// 创建数据包元数据
		timestamp := time.Unix(int64(flowItem.Time/1000), int64((flowItem.Time%1000)*1000000))
		ci := gopacket.CaptureInfo{
			Timestamp:     timestamp,
			CaptureLength: len(buf.Bytes()),
			Length:        len(buf.Bytes()),
		}

		// 写入pcap文件
		err = writer.WritePacket(ci, buf.Bytes())
		if err != nil {
			log.Println("Write packet to pcap failed:", err)
			continue
		}
	}

	return pcapFile
}

// SaveFlowAsJson 将流量数据保存为JSON格式文件
func SaveFlowAsJson(entry FlowEntry) string {
	flowUUID := uuid.New().String()

	// 根据压缩设置确定文件扩展名
	var jsonFile string
	if config.Server_pcap_zip {
		jsonFile = filepath.Join("flow", flowUUID+".json.gz")
	} else {
		jsonFile = filepath.Join("flow", flowUUID+".json")
	}

	// 将FlowEntry转换为JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Println("Marshal JSON failed:", err)
		return ""
	}

	// 创建文件
	file, err := os.Create(jsonFile)
	if err != nil {
		log.Println("Create JSON file failed:", err)
		return ""
	}
	defer file.Close()

	if config.Server_pcap_zip {
		// 创建gzip writer
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		_, err = gzWriter.Write(jsonData)
	} else {
		_, err = file.Write(jsonData)
	}

	if err != nil {
		log.Println("Write JSON file failed:", err)
		return ""
	}

	return jsonFile
}

func reassemblyCallback(entry FlowEntry) {
	ParseHttpFlow(&entry)
	if flag_regex != "" {
		ApplyFlagTags(&entry, flag_regex)
	}
	// B64字段已经在tcp.go中设置，这里不需要额外处理
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

	// 保存流量数据为JSON格式
	jsonFile := SaveFlowAsJson(entry)
	if jsonFile == "" {
		log.Println("Failed to save JSON file for flow")
		return
	}

	// 保存TCP流为pcap格式
	pcapFile := SaveFlowAsPcap(entry)
	if pcapFile == "" {
		log.Println("Failed to save pcap file for flow")
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
		FlowFile:     jsonFile, // JSON文件路径
		PcapFile:     pcapFile, // PCAP文件路径
		Tags:         string(Tags),
		Size:         entry.Size,
	}
	err = config.Db.Create(&pcapRecord).Error

	if err != nil {
		log.Fatalf("Failed to insert pcap record into database: %v", err)
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
	// 确保flow目录存在
	flowDir := "flow/"
	if _, err := os.Stat(flowDir); os.IsNotExist(err) {
		log.Printf("Flow directory %s does not exist, creating...", flowDir)
		err := os.MkdirAll(flowDir, os.ModePerm)
		if err != nil {
			log.Printf("Failed to create flow directory %s: %v", flowDir, err)
			return
		}
		log.Printf("Successfully created flow directory: %s", flowDir)
	}

	// 确保监控目录存在
	if _, err := os.Stat(watch_dir); os.IsNotExist(err) {
		log.Printf("Watch directory %s does not exist, creating...", watch_dir)
		err := os.MkdirAll(watch_dir, os.ModePerm)
		if err != nil {
			log.Printf("Failed to create watch directory %s: %v", watch_dir, err)
			return
		}
		log.Printf("Successfully created watch directory: %s", watch_dir)
	}

	// 验证目录状态
	stat, err := os.Stat(watch_dir)
	if err != nil {
		log.Printf("Failed to stat watch directory %s: %v", watch_dir, err)
		return
	}

	if !stat.IsDir() {
		log.Printf("Watch path %s is not a directory", watch_dir)
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
// checkFileByMD5 检查数据库中是否已存在相同MD5的文件
func checkFileByMD5(fileMD5 string) bool {
	var pcapFile database.PcapFile
	err := config.Db.Where("md5 = ?", fileMD5).First(&pcapFile).Error
	if err != nil {
		// 数据库中没有找到相同MD5的文件
		return false
	}
	log.Printf("File with MD5 %s already exists in database: %s", fileMD5, pcapFile.Filename)
	return true
}

func checkfile(fname string, status bool) bool {
	// 获取文件信息
	fileInfo, err := os.Stat(fname)
	if err != nil {
		log.Println("Failed to get file info:", err)
		return false
	}

	fileModTime := fileInfo.ModTime()
	fileSize := fileInfo.Size()

	// 计算文件MD5
	fileMD5, err := calculateFileMD5(fname)
	if err != nil {
		log.Println("Failed to calculate file MD5:", err)
		return false
	}

	// 首先检查数据库中是否已存在相同MD5的文件
	if checkFileByMD5(fileMD5) {
		return false // 已存在相同MD5的文件，跳过处理
	}

	// 查询数据库中是否存在该文件记录
	var pcapFile database.PcapFile
	err = config.Db.Where("filename = ?", fname).First(&pcapFile).Error
	if err != nil {
		// 如果数据库中没有记录，需要处理
		if status {
			pcapFile := database.PcapFile{
				Filename: fname,
				ModTime:  fileModTime,
				FileSize: fileSize,
				MD5:      fileMD5,
			}
			err = config.Db.Create(&pcapFile).Error
			if err != nil {
				log.Println("Failed to insert pcap file:", err)
			}
		}
		return true
	}

	// 检查MD5是否匹配，如果MD5相同则不需要重新处理
	if pcapFile.MD5 == fileMD5 {
		log.Printf("File %s has same MD5 (%s), skipping processing", fname, fileMD5)
		return false
	}

	// 检查修改时间和文件大小是否匹配
	if !pcapFile.ModTime.Equal(fileModTime) || pcapFile.FileSize != fileSize {
		// 文件已修改，需要重新处理
		if status {
			// 更新数据库中的文件信息
			err = config.Db.Model(&pcapFile).Updates(map[string]interface{}{
				"mod_time":  fileModTime,
				"file_size": fileSize,
				"md5":       fileMD5,
			}).Error
			if err != nil {
				log.Println("Failed to update pcap file info:", err)
			}
		}
		return true
	}

	// 文件未修改，不需要重新处理
	return false
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
	default:
		source = gopacket.NewPacketSource(handle, linktype)
	}

	source.Lazy = lazy
	source.NoCopy = true
	count := 0
	bytes := int64(0)
	tcpCount := 0
	udpCount := 0
	otherCount := 0
	defragger := ip4defrag.NewIPv4Defragmenter()

	streamFactory := &tcpStreamFactory{source: fname, reassemblyCallback: reassemblyCallback}
	streamPool := reassembly.NewStreamPool(streamFactory)
	assembler := reassembly.NewAssembler(streamPool)

	// 创建UDP流工厂
	udpFactory := newUDPStreamFactory(fname, reassemblyCallback)

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

		// 定期清理超时的UDP流
		if count%1000 == 0 { // 每1000个数据包清理一次
			udpFactory.cleanupExpiredStreams()
		}

		// defrag the IPv4 packet if required
		ip4Layer := packet.Layer(layers.LayerTypeIPv4)
		if !nodefrag && ip4Layer != nil {
			ip4 := ip4Layer.(*layers.IPv4)
			l := ip4.Length
			newip4, err := defragger.DefragIPv4(ip4)
			if err != nil {
				log.Printf("Error while de-fragmenting IPv4: %v", err)
				continue
			} else if newip4 == nil {
				continue // packet fragment, we don't have whole packet yet.
			}
			if newip4.Length != l {
				pb, ok := packet.(gopacket.PacketBuilder)
				if !ok {
					log.Printf("Packet is not a PacketBuilder, skipping")
					continue
				}
				nextDecoder := newip4.NextLayerType()
				nextDecoder.Decode(newip4.Payload, pb)
			}
		}

		// 处理IPv6分片（简化版本）
		ip6Layer := packet.Layer(layers.LayerTypeIPv6)
		if !nodefrag && ip6Layer != nil {
			ip6 := ip6Layer.(*layers.IPv6)
			// 检查是否是分片
			if ip6.NextHeader == layers.IPProtocolIPv6Fragment {
				// 对于IPv6分片，我们暂时跳过，因为需要更复杂的重组逻辑
				log.Printf("IPv6 fragment detected, skipping for now")
				continue
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
			tcpCount++
		case layers.LayerTypeUDP:
			// 处理UDP数据包
			udpFactory.ProcessPacket(packet)
			udpCount++
		default:
			// 其他协议暂时忽略
			otherCount++
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

	// 完成所有UDP流处理
	udpFactory.FlushAll()

	// 输出统计信息
	log.Printf("PCAP processing completed for %s:", fname)
	log.Printf("  Total packets: %d", count)
	log.Printf("  TCP packets: %d", tcpCount)
	log.Printf("  UDP packets: %d", udpCount)
	log.Printf("  Other packets: %d", otherCount)
	log.Printf("  Total bytes: %d", bytes)

	checkfile(fname, true)
}
