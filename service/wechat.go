package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// ============================================================
// 微信公众号带参数二维码扫码登录服务
// ============================================================

// ScanStatus 扫码状态
type ScanStatus string

const (
	ScanStatusWaiting  ScanStatus = "waiting"  // 等待扫码
	ScanStatusScanned  ScanStatus = "scanned"  // 已扫码，待确认
	ScanStatusConfirmed ScanStatus = "confirmed" // 已确认，可登录
	ScanStatusExpired  ScanStatus = "expired"  // 已过期
)

// ScanRecord 扫码记录
type ScanRecord struct {
	Status     ScanStatus `json:"status"`
	OpenID     string     `json:"openid"`
	SceneStr   string     `json:"scene_str"`
	CreatedAt  int64      `json:"created_at"`
	BindMode   bool       `json:"bind_mode"`   // 是否绑定模式
	UserID     int        `json:"user_id"`      // 绑定模式下的用户ID
}

// WeChatQRCodeResponse 创建二维码的微信API响应
type WeChatQRCodeResponse struct {
	Ticket        string `json:"ticket"`
	ExpireSeconds int    `json:"expire_seconds"`
	URL           string `json:"url"`
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
}

// WeChatAccessTokenResponse access_token API响应
type WeChatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// WeChatEventMessage 微信事件推送消息
type WeChatEventMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Ticket       string   `xml:"Ticket"`
}

// WeChatEncryptedMessage 微信加密消息（消息加解密模式下收到的XML）
type WeChatEncryptedMessage struct {
	XMLName    xml.Name `xml:"xml"`
	ToUserName string   `xml:"ToUserName"`
	Encrypt    string   `xml:"Encrypt"`
}

// scanStore 扫码状态存储
var (
	scanStore     = make(map[string]*ScanRecord)
	scanStoreLock sync.RWMutex
)

// accessTokenCache access_token缓存
var (
	accessTokenValue   string
	accessTokenExpireAt int64
	accessTokenLock    sync.Mutex
)

// QRCodeExpireSeconds 二维码有效期（秒）
const QRCodeExpireSeconds = 300

// ScanRecordExpireSeconds 扫码记录有效期（秒）
const ScanRecordExpireSeconds = 360

// IsWeChatOffiAccountEnabled 判断是否启用了公众号带参数二维码模式
func IsWeChatOffiAccountEnabled() bool {
	return common.WeChatOffiAccountAppID != "" && common.WeChatOffiAccountAppSecret != ""
}

// ============================================================
// access_token 管理
// ============================================================

// GetWeChatAccessToken 获取微信公众号access_token（带缓存）
func GetWeChatAccessToken() (string, error) {
	accessTokenLock.Lock()
	defer accessTokenLock.Unlock()

	// 检查缓存是否有效（提前200秒刷新）
	if accessTokenValue != "" && time.Now().Unix() < accessTokenExpireAt-200 {
		return accessTokenValue, nil
	}

	// 请求新的access_token
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		common.WeChatOffiAccountAppID, common.WeChatOffiAccountAppSecret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("请求微信access_token失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取access_token响应失败: %v", err)
	}

	var result WeChatAccessTokenResponse
	if err := parseJSON(body, &result); err != nil {
		return "", fmt.Errorf("解析access_token响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取access_token失败: [%d] %s", result.ErrCode, result.ErrMsg)
	}

	// 缓存access_token
	accessTokenValue = result.AccessToken
	accessTokenExpireAt = time.Now().Unix() + int64(result.ExpiresIn)

	common.SysLog(fmt.Sprintf("微信access_token已更新，有效期: %d秒", result.ExpiresIn))

	return accessTokenValue, nil
}

// ============================================================
// 二维码创建
// ============================================================

// CreateWeChatQRCode 创建带参数的临时二维码
func CreateWeChatQRCode(sceneStr string) (string, error) {
	accessToken, err := GetWeChatAccessToken()
	if err != nil {
		return "", err
	}

	// 创建临时二维码请求
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token=%s", accessToken)

	requestBody := fmt.Sprintf(`{"expire_seconds": %d, "action_name": "QR_STR_SCENE", "action_info": {"scene": {"scene_str": "%s"}}}`,
		QRCodeExpireSeconds, sceneStr)

	resp, err := http.Post(url, "application/json", strings.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("请求微信创建二维码失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取创建二维码响应失败: %v", err)
	}

	var result WeChatQRCodeResponse
	if err := parseJSON(body, &result); err != nil {
		return "", fmt.Errorf("解析创建二维码响应失败: %v", err)
	}

	if result.ErrCode != 0 {
		// access_token过期时清除缓存重试
		if result.ErrCode == 40001 || result.ErrCode == 42001 {
			accessTokenLock.Lock()
			accessTokenValue = ""
			accessTokenExpireAt = 0
			accessTokenLock.Unlock()
		}
		return "", fmt.Errorf("创建二维码失败: [%d] %s", result.ErrCode, result.ErrMsg)
	}

	// 返回二维码图片URL
	qrcodeURL := fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket=%s", encodeTicket(result.Ticket))
	return qrcodeURL, nil
}

// ============================================================
// 扫码状态管理
// ============================================================

// GenerateSceneStr 生成唯一的scene_str
func GenerateSceneStr() string {
	return fmt.Sprintf("login_%d_%s", time.Now().UnixMilli(), common.GetRandomString(8))
}

// SetScanRecord 保存扫码记录
func SetScanRecord(sceneStr string, record *ScanRecord) {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	record.SceneStr = sceneStr
	record.CreatedAt = time.Now().Unix()
	scanStore[sceneStr] = record
}

// GetScanRecord 获取扫码记录
func GetScanRecord(sceneStr string) (*ScanRecord, bool) {
	scanStoreLock.RLock()
	defer scanStoreLock.RUnlock()
	record, ok := scanStore[sceneStr]
	if !ok {
		return nil, false
	}
	// 检查是否过期
	if time.Now().Unix()-record.CreatedAt > ScanRecordExpireSeconds {
		return nil, false
	}
	return record, true
}

// DeleteScanRecord 删除扫码记录
func DeleteScanRecord(sceneStr string) {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	delete(scanStore, sceneStr)
}

// UpdateScanStatus 更新扫码状态
func UpdateScanStatus(sceneStr string, status ScanStatus, openID string) {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	if record, ok := scanStore[sceneStr]; ok {
		record.Status = status
		if openID != "" {
			record.OpenID = openID
		}
	}
}

// CleanExpiredScanRecords 清理过期的扫码记录
func CleanExpiredScanRecords() {
	scanStoreLock.Lock()
	defer scanStoreLock.Unlock()
	now := time.Now().Unix()
	for key, record := range scanStore {
		if now-record.CreatedAt > ScanRecordExpireSeconds {
			delete(scanStore, key)
		}
	}
}

// StartScanRecordCleaner 启动定时清理任务
func StartScanRecordCleaner() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			CleanExpiredScanRecords()
		}
	}()
}

// ============================================================
// 微信回调验证
// ============================================================

// VerifyWeChatCallback 验证微信回调URL的signature
func VerifyWeChatCallback(signature, timestamp, nonce string) bool {
	token := common.WeChatOffiAccountToken
	if token == "" {
		return false
	}

	// 将token、timestamp、nonce三个参数进行字典序排序
	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)

	// 将三个参数字符串拼接成一个字符串进行sha1加密
	h := sha1.New()
	for _, s := range strs {
		h.Write([]byte(s))
	}
	calculated := hex.EncodeToString(h.Sum(nil))

	return calculated == signature
}

// ParseWeChatCallback 解析微信事件推送（支持加密模式）
func ParseWeChatCallback(body []byte, timestamp, nonce string) (*WeChatEventMessage, error) {
	// 先尝试解析为加密消息
	var encMsg WeChatEncryptedMessage
	if err := xml.Unmarshal(body, &encMsg); err == nil && encMsg.Encrypt != "" {
		// 加密模式：需要解密
		decryptedXML, err := decryptWeChatMessage(encMsg.Encrypt, timestamp, nonce)
		if err != nil {
			return nil, fmt.Errorf("解密微信消息失败: %v", err)
		}
		// 解密成功后，再解析明文XML
		var msg WeChatEventMessage
		if err := xml.Unmarshal([]byte(decryptedXML), &msg); err != nil {
			return nil, fmt.Errorf("解析解密后的消息失败: %v", err)
		}
		return &msg, nil
	}

	// 明文模式：直接解析
	var msg WeChatEventMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("解析微信事件推送失败: %v", err)
	}
	return &msg, nil
}

// ============================================================
// 微信消息加解密
// ============================================================

// getAESKey 从 EncodingAESKey 解码得到 AES Key
// EncodingAESKey = Base64(64位) => 补充 "=" 后 Base64 解码 => 32字节 AES Key
func getAESKey() ([]byte, error) {
	encodingKey := common.WeChatOffiAccountEncodingAESKey
	if encodingKey == "" {
		return nil, fmt.Errorf("未配置 WeChatOffiAccountEncodingAESKey")
	}
	// 微信的 EncodingAESKey 是 43 位，Base64 解码时需补 "="
	padded := encodingKey + "="
	key, err := base64.StdEncoding.DecodeString(padded)
	if err != nil {
		return nil, fmt.Errorf("Base64解码EncodingAESKey失败: %v", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("AES Key长度错误: 期望32字节, 实际%d字节", len(key))
	}
	return key, nil
}

// verifyMsgSignature 验证消息签名
// msg_signature = sha1(sort(token, timestamp, nonce, encrypt))
func verifyMsgSignature(timestamp, nonce, encrypt, msgSignature string) bool {
	token := common.WeChatOffiAccountToken
	strs := []string{token, timestamp, nonce, encrypt}
	sort.Strings(strs)

	h := sha1.New()
	for _, s := range strs {
		h.Write([]byte(s))
	}
	calculated := hex.EncodeToString(h.Sum(nil))
	return calculated == msgSignature
}

// VerifyWeChatMsgSignature 导出的消息签名验证函数
func VerifyWeChatMsgSignature(timestamp, nonce, encrypt, msgSignature string) bool {
	return verifyMsgSignature(timestamp, nonce, encrypt, msgSignature)
}

// decryptWeChatMessage 解密微信加密消息
func decryptWeChatMessage(encrypt, timestamp, nonce string) (string, error) {
	// 验证签名（如果有 msg_signature 参数的话，这里从timestamp/nonce参数传入）
	// 注意：签名验证在controller层通过query参数完成，这里跳过

	key, err := getAESKey()
	if err != nil {
		return "", err
	}

	// Base64 解码密文
	ciphertext, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil {
		return "", fmt.Errorf("Base64解码密文失败: %v", err)
	}

	// AES-CBC 解密
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建AES cipher失败: %v", err)
	}

	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("密文长度不合法: %d", len(ciphertext))
	}

	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 去除 PKCS#7 填充
	plaintext = pkcs7Unpad(plaintext)

	// 明文格式：random(16字节) + msgLen(4字节大端) + msg + appId
	if len(plaintext) < 20 {
		return "", fmt.Errorf("解密后明文长度不足")
	}

	msgLen := binary.BigEndian.Uint32(plaintext[16:20])
	if int(20+msgLen) > len(plaintext) {
		return "", fmt.Errorf("消息长度不匹配: 期望%d, 实际可用%d", msgLen, len(plaintext)-20)
	}

	msg := plaintext[20 : 20+msgLen]
	receivedAppID := plaintext[20+msgLen:]

	// 验证 appId
	if string(receivedAppID) != common.WeChatOffiAccountAppID {
		common.SysLog(fmt.Sprintf("微信消息解密: appId不匹配, 收到=%s, 期望=%s", string(receivedAppID), common.WeChatOffiAccountAppID))
	}

	return string(msg), nil
}

// pkcs7Unpad 去除 PKCS#7 填充
func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > aes.BlockSize || padding > len(data) {
		return data
	}
	// 验证填充
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return data
		}
	}
	return data[:len(data)-padding]
}

// HandleWeChatEvent 处理微信事件
func HandleWeChatEvent(msg *WeChatEventMessage) {
	// 提取scene_str
	// 关注事件: EventKey = "qrscene_xxx"
	// 扫码事件(已关注): EventKey = "xxx"
	sceneStr, _ := strings.CutPrefix(msg.EventKey, "qrscene_")
	common.SysLog(fmt.Sprintf("微信事件推送: EventKey=%s, sceneStr=%s", msg.EventKey, sceneStr))

	if sceneStr == "" {
		common.SysLog("微信事件推送: sceneStr为空，跳过处理")
		return
	}

	openID := msg.FromUserName

	// 检查扫码记录是否存在
	_, ok := GetScanRecord(sceneStr)
	if !ok {
		common.SysLog(fmt.Sprintf("微信事件推送: 未找到scene_str=%s的记录", sceneStr))
		return
	}

	// 更新状态
	switch msg.Event {
	case "subscribe", "SCAN":
		// 用户扫码关注或已关注用户扫码
		UpdateScanStatus(sceneStr, ScanStatusConfirmed, openID)
		common.SysLog(fmt.Sprintf("微信扫码成功: scene_str=%s, openid=%s, event=%s", sceneStr, openID, msg.Event))
	default:
		common.SysLog(fmt.Sprintf("微信事件推送: 未处理的事件类型=%s, scene_str=%s", msg.Event, sceneStr))
	}
}

// ============================================================
// 辅助函数
// ============================================================

// parseJSON 解析JSON
func parseJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// encodeTicket 对ticket进行URL编码
func encodeTicket(ticket string) string {
	return strings.ReplaceAll(ticket, "+", "%2B")
}
