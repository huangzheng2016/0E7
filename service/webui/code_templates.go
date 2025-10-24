package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 代码生成模板类型
type CodeTemplateType string

const (
	TemplateRequests CodeTemplateType = "requests"
	TemplatePwntools CodeTemplateType = "pwntools"
	TemplateCurl     CodeTemplateType = "curl"
)

// 代码生成请求结构
type CodeGenerateRequest struct {
	PcapId   int              `json:"pcap_id"`
	Template CodeTemplateType `json:"template"`
	FlowData []FlowItem       `json:"flow_data"`
}

// 流量项结构
type FlowItem struct {
	F string `json:"f"` // from: 'c' for client, 's' for server
	B string `json:"b"` // base64 data
	T int64  `json:"t"` // time
}

// 生成代码
func generateCode(pcapId int, templateType CodeTemplateType, flowData []FlowItem) (string, error) {
	// 获取pcap信息
	var pcap database.Pcap
	err := config.Db.Where("id = ?", pcapId).First(&pcap).Error
	if err != nil {
		return "", fmt.Errorf("pcap not found: %v", err)
	}

	// 解析流量数据，只处理客户端到服务器的请求
	var requestFlow *FlowItem
	for _, flow := range flowData {
		if flow.F == "c" { // 客户端请求
			requestFlow = &flow
			break
		}
	}

	if requestFlow == nil {
		return "", fmt.Errorf("no client request found in flow data")
	}

	// 解码base64数据
	decodedData, err := base64.StdEncoding.DecodeString(requestFlow.B)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %v", err)
	}

	// 解析HTTP请求
	httpData := string(decodedData)
	lines := strings.Split(httpData, "\n")

	var path string
	var headers = make(map[string]string)
	var body string
	var inBody bool

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")

		if i == 0 {
			// 解析请求行
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				path = parts[1]
			}
		} else if line == "" {
			// 空行，开始解析body
			inBody = true
		} else if !inBody {
			// 解析头部
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				headers[key] = value
			}
		} else {
			// body内容
			body += line + "\n"
		}
	}

	// 构建URL
	url := fmt.Sprintf("http://%s:%s%s", pcap.DstIP, pcap.DstPort, path)
	host := pcap.DstIP
	port, _ := strconv.Atoi(pcap.DstPort)

	// 根据模板类型生成代码
	// 对于pwntools模板，传递原始数据而不是base64编码的数据
	var dataToPass string
	if templateType == TemplatePwntools {
		dataToPass = string(decodedData)
	} else {
		dataToPass = requestFlow.B
	}
	return generateCodeFromTemplate(templateType, url, host, port, headers, dataToPass, string(decodedData))
}

// 从数据库模板生成代码
func generateCodeFromTemplate(templateType CodeTemplateType, url, host string, port int, headers map[string]string, base64Data string, rawData string) (string, error) {
	// 根据模板类型获取模板名称
	var templateName string
	switch templateType {
	case TemplateRequests:
		templateName = "requests_template"
	case TemplatePwntools:
		templateName = "pwntools_template"
	case TemplateCurl:
		templateName = "curl_template"
	default:
		return "", fmt.Errorf("unsupported template type: %s", templateType)
	}

	// 从数据库获取模板
	var action database.Action
	err := config.Db.Where("name = ?", templateName).First(&action).Error
	if err != nil {
		return "", fmt.Errorf("template not found: %v", err)
	}

	// 解码模板代码
	templateCode, err := decodeActionCode(action.Code)
	if err != nil {
		return "", fmt.Errorf("failed to decode template: %v", err)
	}

	// 准备模板数据
	headersJson, err := json.MarshalIndent(headers, "", "    ")
	if err != nil {
		return "", err
	}

	// 为curl生成headers字符串
	var headersCurl strings.Builder
	for key, value := range headers {
		headersCurl.WriteString(fmt.Sprintf("  -H \"%s: %s\" \\\n", key, value))
	}

	// 对原始数据进行转义，确保在模板中正确显示
	escapedRawData := strings.ReplaceAll(rawData, "\\", "\\\\")       // 转义反斜杠
	escapedRawData = strings.ReplaceAll(escapedRawData, "\"", "\\\"") // 转义双引号
	escapedRawData = strings.ReplaceAll(escapedRawData, "\n", "\\n")  // 转义换行符
	escapedRawData = strings.ReplaceAll(escapedRawData, "\r", "\\r")  // 转义回车符
	escapedRawData = strings.ReplaceAll(escapedRawData, "\t", "\\t")  // 转义制表符

	// 构建模板变量
	templateData := map[string]interface{}{
		"URL":         url,
		"Host":        host,
		"Port":        port,
		"Headers":     string(headersJson),
		"Data":        base64Data,
		"RawData":     escapedRawData, // 转义后的原始数据
		"HeadersMap":  headers,
		"HeadersCurl": headersCurl.String(),
	}

	// 简单的模板替换
	result := templateCode
	for key, value := range templateData {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	return result, nil
}

// 解码Action代码
func decodeActionCode(code string) (string, error) {
	// 检查是否是base64编码的代码
	if strings.HasPrefix(code, "data:code/python3;base64,") {
		base64Data := strings.TrimPrefix(code, "data:code/python3;base64,")
		decoded, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}
	// 如果不是base64格式，直接返回
	return code, nil
}

// 代码生成API接口
func pcap_generate_code(c *gin.Context) {
	pcapIdStr := c.PostForm("pcap_id")
	templateType := c.PostForm("template")
	flowDataStr := c.PostForm("flow_data")

	if pcapIdStr == "" || templateType == "" || flowDataStr == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "缺少必要参数",
		})
		return
	}

	pcapId, err := strconv.Atoi(pcapIdStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "无效的pcap_id",
		})
		return
	}

	// 解析流量数据
	var flowData []FlowItem
	err = json.Unmarshal([]byte(flowDataStr), &flowData)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "无效的流量数据格式",
		})
		return
	}

	// 生成代码
	code, err := generateCode(pcapId, CodeTemplateType(templateType), flowData)
	if err != nil {
		c.JSON(500, gin.H{
			"message": "fail",
			"error":   "代码生成失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result": gin.H{
			"code":     code,
			"template": templateType,
		},
	})
}
