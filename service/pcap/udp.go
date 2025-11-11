package pcap

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// UDP流结构
type udpStream struct {
	net, transport     gopacket.Flow
	source             string
	FlowItems          []FlowItem
	src_port           layers.UDPPort
	dst_port           layers.UDPPort
	total_size         int
	num_packets        int
	last_seen          time.Time
	timeout            time.Duration
	reassemblyCallback func(FlowEntry)
	// 保存所有原始数据包，用于Wireshark分析
	originalPackets [][]byte
	linktype        layers.LinkType
	sync.Mutex
}

// UDP流工厂
type udpStreamFactory struct {
	source             string
	reassemblyCallback func(FlowEntry)
	streams            map[string]*udpStream
	timeout            time.Duration
	sync.Mutex
	linktype layers.LinkType
}

// 创建新的UDP流工厂
func newUDPStreamFactory(source string, reassemblyCallback func(FlowEntry)) *udpStreamFactory {
	return &udpStreamFactory{
		source:             source,
		reassemblyCallback: reassemblyCallback,
		streams:            make(map[string]*udpStream),
		timeout:            30 * time.Second, // UDP流超时时间
	}
}

// 获取或创建UDP流
func (factory *udpStreamFactory) getOrCreateStream(net, transport gopacket.Flow, udp *layers.UDP) *udpStream {
	factory.Lock()
	defer factory.Unlock()

	// 创建流的唯一标识符
	streamKey := net.String() + ":" + transport.String()

	stream, exists := factory.streams[streamKey]
	if !exists {
		stream = &udpStream{
			net:                net,
			transport:          transport,
			source:             factory.source,
			FlowItems:          []FlowItem{},
			src_port:           udp.SrcPort,
			dst_port:           udp.DstPort,
			reassemblyCallback: factory.reassemblyCallback,
			timeout:            factory.timeout,
			last_seen:          time.Now(),
			linktype:           factory.linktype,
		}
		factory.streams[streamKey] = stream
	}

	stream.last_seen = time.Now()
	return stream
}

// 处理UDP数据包
func (factory *udpStreamFactory) ProcessPacket(packet gopacket.Packet) {
	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return
	}

	udp := udpLayer.(*layers.UDP)
	netLayer := packet.NetworkLayer()
	if netLayer == nil {
		return
	}

	net := netLayer.NetworkFlow()
	transport := gopacket.NewFlow(layers.EndpointUDPPort, []byte{byte(udp.SrcPort >> 8), byte(udp.SrcPort)}, []byte{byte(udp.DstPort >> 8), byte(udp.DstPort)})

	stream := factory.getOrCreateStream(net, transport, udp)
	stream.addPacket(packet, udp)
}

// 向UDP流添加数据包
func (s *udpStream) addPacket(packet gopacket.Packet, udp *layers.UDP) {
	s.Lock()
	defer s.Unlock()

	s.num_packets++

	// 获取数据包时间戳
	timestamp := packet.Metadata().CaptureInfo.Timestamp

	// 确定数据方向：基于当前数据包与流初始方向对比
	var from string
	if nl := packet.NetworkLayer(); nl != nil {
		curNet := nl.NetworkFlow()
		if curNet.Src().String() == s.net.Src().String() {
			from = "c" // 客户端到服务器
		} else {
			from = "s" // 服务器到客户端
		}
	} else {
		from = "c"
	}

	// 获取UDP载荷数据
	payload := udp.Payload
	if len(payload) == 0 {
		return
	}

	// 保存原始数据包到流中（设置上限以避免内存过大）
	const maxOriginalPackets = 1000
	if len(s.originalPackets) < maxOriginalPackets {
		dataCopy := make([]byte, len(packet.Data()))
		copy(dataCopy, packet.Data())
		s.originalPackets = append(s.originalPackets, dataCopy)
	}

	flowItem := FlowItem{
		B64:  base64.StdEncoding.EncodeToString(payload),
		From: from,
		Time: int(timestamp.UnixNano() / 1000000),
	}

	s.FlowItems = append(s.FlowItems, flowItem)
	s.total_size += len(payload)
}

// 检查并清理超时的UDP流
func (factory *udpStreamFactory) cleanupExpiredStreams() {
	factory.Lock()
	defer factory.Unlock()

	now := time.Now()
	for key, stream := range factory.streams {
		if now.Sub(stream.last_seen) > stream.timeout {
			// 流已超时，处理并删除
			stream.finalize()
			delete(factory.streams, key)
		}
	}
}

// 完成UDP流处理
func (s *udpStream) finalize() {
	if len(s.FlowItems) == 0 {
		return
	}

	// 找到最小和最大时间戳来计算准确的持续时间
	minTime := s.FlowItems[0].Time
	maxTime := s.FlowItems[0].Time

	for _, item := range s.FlowItems {
		if item.Time < minTime {
			minTime = item.Time
		}
		if item.Time > maxTime {
			maxTime = item.Time
		}
	}

	time := minTime
	duration := maxTime - minTime

	// 获取网络端点信息
	src, dst := s.net.Endpoints()

	// 创建FlowEntry
	entry := FlowEntry{
		SrcPort:         int(s.src_port),
		DstPort:         int(s.dst_port),
		SrcIp:           src.String(),
		DstIp:           dst.String(),
		Time:            time,
		Duration:        duration,
		NumPackets:      s.num_packets,
		Blocked:         false,
		Filename:        s.source,
		Flow:            s.FlowItems,
		Size:            s.total_size,
		OriginalPackets: s.originalPackets,
		LinkType:        s.linktype,
		Tags:            []string{"UDP"},
	}

	// 调用重组回调函数
	s.reassemblyCallback(entry)
}

// 读取工厂的链路类型（简化访问）
// 删除占位方法（不再需要）

// 强制完成所有UDP流
func (factory *udpStreamFactory) FlushAll() {
	factory.Lock()
	defer factory.Unlock()

	for _, stream := range factory.streams {
		stream.finalize()
	}
	factory.streams = make(map[string]*udpStream)
}
