package pcap

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/service/search"
	"bytes"
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
	"runtime"
	"strings"
	"sync"
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

	// 文件处理队列
	pcapFileChan chan pcapFileTask
	pcapWg       sync.WaitGroup
	pcapWorkers  int
	queueMutex   sync.RWMutex
	queueStarted bool
)

// pcapFileTask 文件处理任务
type pcapFileTask struct {
	filePath string
	checkMD5 bool // 是否需要进行MD5检查
}

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

// 使用 search 包中的 FlowItem 类型
type FlowItem = search.FlowItem

type FlowEntry struct {
	SrcPort    int
	DstPort    int
	SrcIp      string
	DstIp      string
	Time       int
	Duration   int
	NumPackets int
	Blocked    bool
	Filename   string
	Flow       []FlowItem
	Tags       []string
	Size       int
	// 保存所有原始数据包（原始字节），仅用于生成PCAP，不参与JSON/DB
	OriginalPackets [][]byte `json:"-"`
	// 捕获的链路类型，用于写入 pcap 头（仅当使用 OriginalPackets 时生效）
	LinkType layers.LinkType `json:"-"`
}

// IPv6 分片重组的最小实现
type ipv6FragmentKey struct {
	src        string
	dst        string
	id         uint32
	nextHeader layers.IPProtocol
}

type ipv6FragmentBuffer struct {
	parts     map[uint32][]byte // offset (in bytes) -> data
	haveStart bool
	haveEnd   bool
	totalLen  int // known when haveEnd
	created   time.Time
	updated   time.Time
}

type ipv6Defragmenter struct {
	frags map[ipv6FragmentKey]*ipv6FragmentBuffer
}

func newIPv6Defragmenter() *ipv6Defragmenter {
	return &ipv6Defragmenter{frags: make(map[ipv6FragmentKey]*ipv6FragmentBuffer)}
}

// DefragIPv6 接收一个 IPv6 包与其 Fragment 头，返回：
// - newip6: 完整重组后的 IPv6 层（若尚未完整则返回 nil）
// - changed: 是否对包体进行了替换
// - err: 错误
func (d *ipv6Defragmenter) DefragIPv6(ip6 *layers.IPv6, frag *layers.IPv6Fragment) (*layers.IPv6, bool, error) {
	key := ipv6FragmentKey{
		src:        ip6.SrcIP.String(),
		dst:        ip6.DstIP.String(),
		id:         frag.Identification,
		nextHeader: frag.NextHeader,
	}

	buf, ok := d.frags[key]
	if !ok {
		buf = &ipv6FragmentBuffer{
			parts:   make(map[uint32][]byte),
			created: time.Now(),
			updated: time.Now(),
		}
		d.frags[key] = buf
	}

	// 计算字节偏移：FragmentOffset 以 8 字节为单位
	offsetBytes := uint32(frag.FragmentOffset) * 8
	// frag.Payload 是该分片的上层负载
	data := make([]byte, len(frag.Payload))
	copy(data, frag.Payload)
	if offsetBytes == 0 {
		buf.haveStart = true
	}
	if !frag.MoreFragments {
		buf.haveEnd = true
		buf.totalLen = int(offsetBytes) + len(data)
	}
	buf.parts[offsetBytes] = data
	buf.updated = time.Now()

	if !(buf.haveStart && buf.haveEnd) {
		return nil, false, nil
	}

	// 检查是否连续完整
	assembled := make([]byte, buf.totalLen)
	filled := 0
	for off := 0; off < buf.totalLen; {
		part, exists := buf.parts[uint32(off)]
		if !exists {
			// 尚未完整
			return nil, false, nil
		}
		copy(assembled[off:], part)
		off += len(part)
		filled += len(part)
	}
	if filled != buf.totalLen {
		return nil, false, nil
	}

	// 构造新的 IPv6 层：NextHeader 使用分片头中的下一层协议
	newip6 := *ip6
	newip6.NextHeader = frag.NextHeader
	newip6.Payload = assembled

	// 清理状态
	delete(d.frags, key)

	return &newip6, true, nil
}

// 清理过期的未完成 IPv6 分片缓存
func (d *ipv6Defragmenter) CleanupExpired(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	for k, v := range d.frags {
		last := v.updated
		if last.IsZero() {
			last = v.created
		}
		if last.Before(cutoff) {
			delete(d.frags, k)
		}
	}
}

// getFlowStoragePath 根据UUID生成分层存储路径
func getFlowStoragePath(uuid string) string {
	// 使用UUID的前2个字符作为第一层目录
	// 使用第3-4个字符作为第二层目录
	if len(uuid) < 4 {
		// 如果UUID太短，使用默认目录
		return filepath.Join("flow", "00", "00")
	}

	firstLevel := uuid[0:2]
	secondLevel := uuid[2:4]

	return filepath.Join("flow", firstLevel, secondLevel)
}

// SaveFlowAsPcap 将TCP流数据保存为pcap格式
// 返回 (文件路径, pcap数据的base64编码)
// 如果pcap大小小于256KB，不写入文件，返回空路径和base64数据
// 如果pcap大小大于256KB，写入文件，返回文件路径和空数据
func SaveFlowAsPcap(entry FlowEntry) (string, string) {
	// 首先生成pcap数据到内存缓冲区
	buf := new(bytes.Buffer)

	var writer *pcapgo.Writer
	var gzWriter *gzip.Writer

	if config.Server_pcap_zip {
		// 使用gzip压缩
		gzWriter = gzip.NewWriter(buf)
		writer = pcapgo.NewWriter(gzWriter)
	} else {
		writer = pcapgo.NewWriter(buf)
	}

	// 写入pcap文件头（优先使用原始 LinkType，如果没有则用以太网）
	headerLinkType := layers.LinkTypeEthernet
	if entry.LinkType != 0 && len(entry.OriginalPackets) > 0 {
		headerLinkType = entry.LinkType
	}
	err := writer.WriteFileHeader(65536, headerLinkType)
	if err != nil {
		log.Println("Write pcap file header failed:", err)
		return "", ""
	}

	// 解析源IP和目标IP
	srcIP := net.ParseIP(entry.SrcIp)
	dstIP := net.ParseIP(entry.DstIp)
	if srcIP == nil || dstIP == nil {
		log.Println("Invalid IP address:", entry.SrcIp, entry.DstIp)
		return "", ""
	}

	// 优先使用原始数据包列表（如果可用）
	if len(entry.OriginalPackets) > 0 {
		// 使用原始数据包列表，保留所有原始layers信息
		for i, packetData := range entry.OriginalPackets {
			// 解析原始数据包以获取正确的时间戳
			packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
			var timestamp time.Time
			if packet.Metadata() != nil && !packet.Metadata().CaptureInfo.Timestamp.IsZero() {
				// 使用原始数据包的时间戳
				timestamp = packet.Metadata().CaptureInfo.Timestamp
			} else if i < len(entry.Flow) {
				// 回退到使用对应FlowItem的时间戳
				timestamp = time.Unix(int64(entry.Flow[i].Time/1000), int64((entry.Flow[i].Time%1000)*1000000))
			} else if len(entry.Flow) > 0 {
				// 如果原始数据包数量超过FlowItem数量，使用最后一个FlowItem的时间戳
				lastFlowItem := entry.Flow[len(entry.Flow)-1]
				timestamp = time.Unix(int64(lastFlowItem.Time/1000), int64((lastFlowItem.Time%1000)*1000000))
			} else {
				timestamp = time.Now()
			}

			ci := gopacket.CaptureInfo{
				Timestamp:     timestamp,
				CaptureLength: len(packetData),
				Length:        len(packetData),
			}

			// 写入pcap数据到缓冲区
			err = writer.WritePacket(ci, packetData)
			if err != nil {
				log.Printf("Write original packet %d to pcap failed: %v", i, err)
				continue
			}
		}

		// 完成写入
		if gzWriter != nil {
			gzWriter.Close()
		}

		// 判断大小
		return savePcapData(buf.Bytes(), entry)
	}

	// 如果没有原始数据包列表，则使用FlowItem重建数据包
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
		packetBuf := gopacket.NewSerializeBuffer()
		opts := gopacket.SerializeOptions{
			ComputeChecksums: true,
			FixLengths:       true,
		}

		// 序列化数据包
		err = gopacket.SerializeLayers(packetBuf, opts,
			ethernet,
			ip,
			tcp,
			gopacket.Payload(tcp.Payload),
		)
		if err != nil {
			log.Println("Serialize packet failed:", err)
			continue
		}

		packetData := packetBuf.Bytes()

		// 创建数据包元数据
		timestamp := time.Unix(int64(flowItem.Time/1000), int64((flowItem.Time%1000)*1000000))
		ci := gopacket.CaptureInfo{
			Timestamp:     timestamp,
			CaptureLength: len(packetData),
			Length:        len(packetData),
		}

		// 写入pcap数据到缓冲区
		err = writer.WritePacket(ci, packetData)
		if err != nil {
			log.Println("Write packet to pcap failed:", err)
			continue
		}
	}

	// 完成写入
	if gzWriter != nil {
		gzWriter.Close()
	}

	// 判断大小并保存
	return savePcapData(buf.Bytes(), entry)
}

// savePcapData 根据pcap数据大小决定存储方式
// 小于256KB：返回空路径和base64编码的数据
// 大于等于256KB：保存到文件，返回文件路径和空数据
func savePcapData(pcapData []byte, entry FlowEntry) (string, string) {
	const sizeThreshold = 256 * 1024 // 256KB

	if len(pcapData) < sizeThreshold {
		// 小文件：不落地，直接返回base64编码的数据
		pcapDataB64 := base64.StdEncoding.EncodeToString(pcapData)
		return "", pcapDataB64
	}

	// 大文件：写入文件
	flowUUID := uuid.New().String()

	// 创建包含流信息的文件名，便于在Wireshark中识别
	flowInfo := fmt.Sprintf("%s_%d_to_%s_%d", entry.SrcIp, entry.SrcPort, entry.DstIp, entry.DstPort)
	// 清理文件名中的特殊字符
	flowInfo = strings.ReplaceAll(flowInfo, ":", "_")
	flowInfo = strings.ReplaceAll(flowInfo, ".", "_")

	// 生成文件名
	var filename string
	if config.Server_pcap_zip {
		filename = fmt.Sprintf("flow_%s_%s.pcap.gz", flowInfo, flowUUID)
	} else {
		filename = fmt.Sprintf("flow_%s_%s.pcap", flowInfo, flowUUID)
	}

	// 生成分层存储路径
	storageDir := getFlowStoragePath(flowUUID)

	// 确保目录存在
	if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
		log.Printf("创建存储目录失败 %s: %v", storageDir, err)
		return "", ""
	}

	pcapFile := filepath.Join(storageDir, filename)

	// 写入文件
	err := ioutil.WriteFile(pcapFile, pcapData, 0644)
	if err != nil {
		log.Printf("写入pcap文件失败: %v", err)
		return "", ""
	}

	return pcapFile, ""
}

// SaveFlowAsJson 将流量数据保存为JSON格式文件
// SaveFlowAsJson 保存flow数据，返回(文件路径, JSON数据)
// 如果flow大小小于256KB，不写入文件，返回空路径和JSON数据
// 如果flow大小大于256KB，写入文件，返回文件路径和空数据
func SaveFlowAsJson(entry FlowEntry) (string, string) {
	// 将FlowEntry转换为JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		log.Println("Marshal JSON failed:", err)
		return "", ""
	}

	// 判断大小，小于256KB直接返回数据，不写文件
	const sizeThreshold = 256 * 1024 // 256KB
	if len(jsonData) < sizeThreshold {
		// 小文件：不落地，直接返回JSON数据
		return "", string(jsonData)
	}

	// 大文件：写入文件
	flowUUID := uuid.New().String()

	// 生成文件名
	var filename string
	if config.Server_pcap_zip {
		filename = flowUUID + ".json.gz"
	} else {
		filename = flowUUID + ".json"
	}

	// 生成分层存储路径
	storageDir := getFlowStoragePath(flowUUID)

	// 确保目录存在
	if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
		log.Printf("创建存储目录失败 %s: %v", storageDir, err)
		return "", ""
	}

	jsonFile := filepath.Join(storageDir, filename)

	// 创建文件
	file, err := os.Create(jsonFile)
	if err != nil {
		log.Println("Create JSON file failed:", err)
		return "", ""
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
		return "", ""
	}

	return jsonFile, ""
}

func reassemblyCallback(entry FlowEntry) {
	ParseHttpFlow(&entry)

	// 所有新流量都先打上PENDING标签，等待后台检测flag
	entry.Tags = append(entry.Tags, "PENDING")

	// B64字段已经在tcp.go中设置，这里不需要额外处理

	Tags, err := json.Marshal(entry.Tags)
	if err != nil {
		log.Println("Tags Error:", err)
		return
	}

	// 保存流量数据为JSON格式
	jsonFile, jsonData := SaveFlowAsJson(entry)
	if jsonFile == "" && jsonData == "" {
		log.Println("Failed to save flow data")
		return
	}

	// 保存TCP流为pcap格式
	pcapFile, pcapData := SaveFlowAsPcap(entry)
	if pcapFile == "" && pcapData == "" {
		log.Println("Failed to save pcap file for flow")
	}

	// 提取客户端和服务器端内容
	var clientContentBuilder, serverContentBuilder strings.Builder
	for _, flowItem := range entry.Flow {
		decoded, err := base64.StdEncoding.DecodeString(flowItem.B64)
		if err != nil {
			log.Printf("解码B64数据失败: %v", err)
			continue
		}
		decodedStr := string(decoded)

		if flowItem.From == "c" {
			// 客户端到服务器
			clientContentBuilder.WriteString(decodedStr)
			clientContentBuilder.WriteString(" ")
		} else if flowItem.From == "s" {
			// 服务器到客户端
			serverContentBuilder.WriteString(decodedStr)
			serverContentBuilder.WriteString(" ")
		}
	}

	pcapRecord := database.Pcap{
		SrcPort:       fmt.Sprintf("%d", entry.SrcPort),
		DstPort:       fmt.Sprintf("%d", entry.DstPort),
		SrcIP:         entry.SrcIp,
		DstIP:         entry.DstIp,
		Time:          entry.Time,
		Duration:      entry.Duration,
		NumPackets:    entry.NumPackets,
		Blocked:       fmt.Sprintf("%t", entry.Blocked),
		Filename:      entry.Filename,
		Tags:          string(Tags),
		ClientContent: clientContentBuilder.String(),
		ServerContent: serverContentBuilder.String(),
		FlowFile:      jsonFile, // JSON文件路径（大文件，>=256KB）
		FlowData:      jsonData, // JSON数据（小文件，<256KB）
		PcapFile:      pcapFile, // PCAP文件路径（大文件，>=256KB）
		PcapData:      pcapData, // PCAP数据（小文件，<256KB，base64编码）
		Size:          entry.Size,
	}
	err = config.Db.Create(&pcapRecord).Error

	if err != nil {
		log.Printf("Failed to insert pcap record into database: %v", err)
		// 不退出程序，继续处理其他记录
		return
	}

	// 不立即建立搜索索引，等待 PENDING 状态处理完成后再索引
	// 搜索索引将在 flag 检测器处理 PENDING 状态时异步建立

}

func Setbpf(str string) {
	bpf = str
}
func SetFlagRegex(flag string) {
	flag_regex = flag
}

// InitPcapQueue 初始化文件处理队列
func InitPcapQueue() {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	if queueStarted {
		return
	}

	// 获取CPU核心数，设置并行处理数量
	pcapWorkers = runtime.NumCPU()
	if pcapWorkers > 8 {
		pcapWorkers = 8 // 限制最大并行数，避免过度并发
	}

	// 可以通过环境变量或配置文件调整并行数
	if config.Server_pcap_workers > 0 && config.Server_pcap_workers <= 32 {
		pcapWorkers = config.Server_pcap_workers
		log.Printf("使用配置的并行处理数量: %d", pcapWorkers)
	}

	// 创建文件处理通道
	pcapFileChan = make(chan pcapFileTask, 8192) // 缓冲区大小为8192

	// 启动并行处理worker
	for i := 0; i < pcapWorkers; i++ {
		pcapWg.Add(1)
		go func(workerID int) {
			defer pcapWg.Done()
			//log.Printf("Pcap处理Worker %d 已启动", workerID)

			for task := range pcapFileChan {
				if task.checkMD5 {
					log.Printf("开始处理文件: %s", task.filePath)
				} else {
					log.Printf("开始处理文件（跳过MD5检查）: %s", task.filePath)
				}
				startTime := time.Now()

				handlePcapUri(task.filePath, bpf, task.checkMD5)

				duration := time.Since(startTime)
				log.Printf("完成处理文件: %s (耗时: %v)", task.filePath, duration)
			}

			//log.Printf("Pcap处理Worker %d 已停止", workerID)
		}(i)
	}

	queueStarted = true
	log.Printf("文件处理队列已启动，%d 个 worker", pcapWorkers)
}

// QueuePcapFile 将 pcap 文件加入处理队列（会进行MD5检查）
func QueuePcapFile(filePath string) {
	queueMutex.RLock()
	defer queueMutex.RUnlock()

	if !queueStarted {
		// 如果队列未启动，直接处理文件
		go handlePcapUri(filePath, bpf, true)
		return
	}

	// 使用阻塞式发送，确保文件不会被丢弃
	// 如果队列满了，这里会阻塞等待直到有空间
	pcapFileChan <- pcapFileTask{filePath: filePath, checkMD5: true}
	log.Printf("已排队处理文件: %s", filePath)
}

// QueuePcapFileSkipCheck 将 pcap 文件加入处理队列（跳过MD5检查）
// 用于文件已经通过MD5检查并保存到数据库后的情况
func QueuePcapFileSkipCheck(filePath string) {
	queueMutex.RLock()
	defer queueMutex.RUnlock()

	if !queueStarted {
		// 如果队列未启动，直接处理文件
		go handlePcapUri(filePath, bpf, false)
		return
	}

	// 使用阻塞式发送，确保文件不会被丢弃
	pcapFileChan <- pcapFileTask{filePath: filePath, checkMD5: false}
	log.Printf("已排队处理文件（跳过MD5检查）: %s", filePath)
}

func ParsePcapfile(fname string, check bool) {
	// 使用队列处理文件
	QueuePcapFile(fname)
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

	// 确保队列已初始化
	InitPcapQueue()

	// 处理现有文件
	files, err := ioutil.ReadDir(watch_dir)
	if err != nil {
		log.Fatal(err)
	}

	// 将现有文件发送到全局处理队列
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".pcap") || strings.HasSuffix(file.Name(), ".pcapng") {
			filePath := filepath.Join(watch_dir, file.Name())
			QueuePcapFile(filePath)
		}
	}

	// 启动文件监控
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	// 文件监控goroutine
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Rename|fsnotify.Create) != 0 {
					if strings.HasSuffix(event.Name, ".pcap") || strings.HasSuffix(event.Name, ".pcapng") {
						log.Println("发现新文件", event.Name, event.Op.String())

						// 等待文件写入完成
						time.Sleep(1 * time.Second)

						// 将新文件发送到全局处理队列
						QueuePcapFile(event.Name)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("文件监控错误:", err)
			}
		}
	}()

	err = watcher.Add(watch_dir)
	if err != nil {
		log.Fatal(err)
	}

	// 保持程序运行
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
				log.Printf("Failed to insert pcap file %s: %v", fname, err)
				return false // 数据库操作失败，返回false
			}
			log.Printf("Successfully inserted pcap file record: %s", fname)
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
				log.Printf("Failed to update pcap file info for %s: %v", fname, err)
				return false // 数据库更新失败，返回false
			}
			log.Printf("Successfully updated pcap file record: %s", fname)
		}
		return true
	}

	// 文件未修改，不需要重新处理
	return false
}
func handlePcapUri(fname string, bpf string, check bool) {
	var handle *pcap.Handle
	var err error

	// 尝试打开pcap文件
	if handle, err = pcap.OpenOffline(fname); err != nil {
		log.Printf("PCAP OpenOffline error: %v", err)

		// 在Windows下，如果pcap库不可用，尝试使用备用方法
		if runtime.GOOS == "windows" {
			log.Println("尝试使用备用pcap解析方法...")
			if err := handlePcapFileFallback(fname, bpf, check); err != nil {
				log.Printf("备用pcap解析也失败: %v", err)
			} else {
				log.Println("成功 使用备用方法成功解析pcap文件")
			}
		}
		return
	}
	defer handle.Close()

	if bpf != "" {
		if err := handle.SetBPFFilter(bpf); err != nil {
			log.Println("Set BPF Filter error: ", err)
			// 即使BPF设置失败，也继续处理文件
		}
	}

	processPcapHandle(handle, fname, check)
}

func processPcapHandle(handle *pcap.Handle, fname string, check bool) {
	if check && !checkfile(fname, false) {
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
	ip6defragger := newIPv6Defragmenter()

	streamFactory := &tcpStreamFactory{source: fname, reassemblyCallback: reassemblyCallback, linktype: linktype}
	streamPool := reassembly.NewStreamPool(streamFactory)
	assembler := reassembly.NewAssembler(streamPool)

	// 创建UDP流工厂
	udpFactory := newUDPStreamFactory(fname, reassemblyCallback)
	udpFactory.linktype = linktype

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
			// IPv6 分片缓存清理（60s 过期）
			ip6defragger.CleanupExpired(60 * time.Second)
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

		// 处理IPv6分片（最小可用重组）
		ip6Layer := packet.Layer(layers.LayerTypeIPv6)
		if !nodefrag && ip6Layer != nil {
			ip6 := ip6Layer.(*layers.IPv6)
			fragLayer := packet.Layer(layers.LayerTypeIPv6Fragment)
			if fragLayer != nil {
				frag := fragLayer.(*layers.IPv6Fragment)
				newip6, changed, err := ip6defragger.DefragIPv6(ip6, frag)
				if err != nil {
					log.Printf("Error while de-fragmenting IPv6: %v", err)
					continue
				} else if newip6 == nil {
					// packet fragment, we don't have whole packet yet.
					continue
				}
				if changed {
					pb, ok := packet.(gopacket.PacketBuilder)
					if !ok {
						log.Printf("Packet is not a PacketBuilder, skipping")
						continue
					}
					nextDecoder := newip6.NextLayerType()
					nextDecoder.Decode(newip6.Payload, pb)
				}
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
				CaptureInfo:    packet.Metadata().CaptureInfo,
				OriginalPacket: packet,
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

	if !checkfile(fname, true) {
		log.Printf("Failed to update file record for %s", fname)
	}
}

// handlePcapFileFallback 备用pcap文件处理方法（当pcap库不可用时使用）
func handlePcapFileFallback(fname string, bpf string, check bool) error {
	log.Printf("使用备用方法处理pcap文件: %s", fname)

	// 检查文件是否存在
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", fname)
	}

	// 尝试使用pcapgo直接读取文件
	file, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	reader, err := pcapgo.NewReader(file)
	if err != nil {
		return fmt.Errorf("无法创建pcap读取器: %v", err)
	}

	// 创建基本的pcap记录
	pcapRecord := database.Pcap{
		Filename:   fname,
		Time:       int(time.Now().Unix()),
		NumPackets: 0,
		Tags:       "fallback_parsed",
	}

	packetCount := 0
	for {
		_, _, err := reader.ReadPacketData()
		if err != nil {
			if err == io.EOF {
				break
			}
			// 忽略单个数据包的错误，继续处理
			continue
		}
		packetCount++

		// 限制处理的数据包数量，避免内存问题
		if packetCount > 10000 {
			log.Printf("达到处理限制，停止处理更多数据包")
			break
		}
	}

	pcapRecord.NumPackets = packetCount

	// 保存到数据库
	if err := config.Db.Create(&pcapRecord).Error; err != nil {
		log.Printf("保存pcap记录失败: %v", err)
	} else {
		log.Printf("成功 成功处理pcap文件，共 %d 个数据包", packetCount)
	}

	return nil
}
