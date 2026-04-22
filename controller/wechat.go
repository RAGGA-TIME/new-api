package controller

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// ============================================================
// 旧模式：公众号验证码登录
// ============================================================

type wechatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func getWeChatIdByCode(code string) (string, error) {
	if code == "" {
		return "", errors.New("无效的参数")
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/wechat/user?code=%s", common.WeChatServerAddress, url.QueryEscape(code)), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", common.WeChatServerToken)
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	httpResponse, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer httpResponse.Body.Close()
	var res wechatLoginResponse
	err = json.NewDecoder(httpResponse.Body).Decode(&res)
	if err != nil {
		return "", err
	}
	if !res.Success {
		return "", errors.New(res.Message)
	}
	if res.Data == "" {
		return "", errors.New("验证码错误或已过期")
	}
	return res.Data, nil
}

// WeChatAuth 旧模式微信登录（验证码方式）
func WeChatAuth(c *gin.Context) {
	if !common.WeChatAuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员未开启通过微信登录以及注册",
			"success": false,
		})
		return
	}

	// 新模式走新逻辑
	if service.IsWeChatOffiAccountEnabled() {
		wechatOffiAccountAuth(c)
		return
	}

	// 旧模式
	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	loginOrRegisterByWeChatId(wechatId, c)
}

// WeChatBind 旧模式微信绑定
func WeChatBind(c *gin.Context) {
	if !common.WeChatAuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员未开启通过微信登录以及注册",
			"success": false,
		})
		return
	}

	// 新模式走新逻辑
	if service.IsWeChatOffiAccountEnabled() {
		wechatOffiAccountBindConfirm(c)
		return
	}

	// 旧模式
	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	bindWeChatIdToCurrentUser(wechatId, c)
}

// ============================================================
// 新模式：公众号带参数二维码扫码登录
// ============================================================

// WeChatGenerateQRCode 生成带参数的临时二维码
func WeChatGenerateQRCode(c *gin.Context) {
	if !common.WeChatAuthEnabled || !service.IsWeChatOffiAccountEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启微信扫码登录",
		})
		return
	}

	// 检查是否绑定模式
	bindMode := c.Query("bind") == "true"

	sceneStr := service.GenerateSceneStr()

	// 保存扫码记录
	record := &service.ScanRecord{
		Status:   service.ScanStatusWaiting,
		BindMode: bindMode,
	}

	// 绑定模式下需要记录用户ID
	if bindMode {
		session := sessions.Default(c)
		id := session.Get("id")
		if id == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "未登录，无法绑定微信",
			})
			return
		}
		record.UserID = id.(int)
	}

	service.SetScanRecord(sceneStr, record)

	// 创建二维码
	qrcodeURL, err := service.CreateWeChatQRCode(sceneStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "创建二维码失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"scene_str":      sceneStr,
		"qrcode_url":     qrcodeURL,
		"expire_seconds": service.QRCodeExpireSeconds,
	})
}

// WeChatScanStatus 查询扫码状态
func WeChatScanStatus(c *gin.Context) {
	if !common.WeChatAuthEnabled || !service.IsWeChatOffiAccountEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启微信扫码登录",
		})
		return
	}

	sceneStr := c.Query("scene_str")
	if sceneStr == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "缺少scene_str参数",
		})
		return
	}

	record, ok := service.GetScanRecord(sceneStr)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"status":  string(service.ScanStatusExpired),
			"message": "二维码已过期，请重新获取",
		})
		return
	}

	// 如果状态还是waiting或scanned，直接返回状态
	if record.Status == service.ScanStatusWaiting || record.Status == service.ScanStatusScanned {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"status":  string(record.Status),
		})
		return
	}

	// 状态为confirmed，执行登录或绑定
	if record.Status == service.ScanStatusConfirmed {
		wechatId := record.OpenID

		if record.BindMode {
			// 绑定模式
			if model.IsWeChatIdAlreadyTaken(wechatId) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"status":  string(record.Status),
					"message": "该微信账号已被绑定",
				})
				service.DeleteScanRecord(sceneStr)
				return
			}

			user := model.User{Id: record.UserID}
			if err := user.FillUserById(); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"status":  string(record.Status),
					"message": "用户不存在",
				})
				service.DeleteScanRecord(sceneStr)
				return
			}

			user.WeChatId = wechatId
			if err := user.Update(false); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"status":  string(record.Status),
					"message": err.Error(),
				})
				service.DeleteScanRecord(sceneStr)
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"status":  string(record.Status),
				"message": "绑定成功",
			})
			service.DeleteScanRecord(sceneStr)
			return
		}

		// 登录模式
		loginOrRegisterByWeChatId(wechatId, c)
		service.DeleteScanRecord(sceneStr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"status":  string(record.Status),
		"message": "二维码已过期",
	})
}

// WeChatCallback 微信事件推送回调（GET验证 + POST事件）
func WeChatCallback(c *gin.Context) {
	if c.Request.Method == http.MethodGet {
		// 微信服务器验证URL
		signature := c.Query("signature")
		timestamp := c.Query("timestamp")
		nonce := c.Query("nonce")
		echostr := c.Query("echostr")

		if service.VerifyWeChatCallback(signature, timestamp, nonce) {
			c.String(http.StatusOK, echostr)
			return
		}
		c.String(http.StatusForbidden, "验证失败")
		return
	}

	// POST: 接收微信事件推送
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.SysLog("读取微信回调请求体失败: " + err.Error())
		c.String(http.StatusOK, "success")
		return
	}
	common.SysLog("微信回调原始请求体: " + string(body))

	// 获取验证签名所需的参数
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	msgSignature := c.Query("msg_signature")

	// 如果是加密模式，验证 msg_signature
	if msgSignature != "" {
		// 先解析出 Encrypt 字段
		var encMsg struct {
			Encrypt string `xml:"Encrypt"`
		}
		if err := xml.Unmarshal(body, &encMsg); err == nil && encMsg.Encrypt != "" {
			if !service.VerifyWeChatMsgSignature(timestamp, nonce, encMsg.Encrypt, msgSignature) {
				common.SysLog("微信回调消息签名验证失败")
				c.String(http.StatusOK, "success")
				return
			}
		}
	}

	msg, err := service.ParseWeChatCallback(body, timestamp, nonce)
	if err != nil {
		common.SysLog("解析微信回调消息失败: " + err.Error())
		c.String(http.StatusOK, "success")
		return
	}
	common.SysLog(fmt.Sprintf("微信回调解析结果: %+v", msg))

	// 处理事件
	service.HandleWeChatEvent(msg)

	// 响应微信服务器
	c.String(http.StatusOK, "success")
}

// ============================================================
// 公共辅助函数
// ============================================================

// loginOrRegisterByWeChatId 通过WeChatId登录或注册
func loginOrRegisterByWeChatId(wechatId string, c *gin.Context) {
	user := model.User{
		WeChatId: wechatId,
	}
	if model.IsWeChatIdAlreadyTaken(wechatId) {
		err := user.FillUserByWeChatId()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		if user.Id == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "用户已注销",
			})
			return
		}
	} else {
		if common.RegisterEnabled {
			user.Username = "wechat_" + strconv.Itoa(model.GetMaxUserId()+1)
			user.DisplayName = "WeChat User"
			user.Role = common.RoleCommonUser
			user.Status = common.UserStatusEnabled

			if err := user.Insert(0); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "管理员关闭了新用户注册",
			})
			return
		}
	}

	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "用户已被封禁",
			"success": false,
		})
		return
	}
	setupLogin(&user, c)
}

// wechatOffiAccountAuth 新模式微信登录（通过openid）
func wechatOffiAccountAuth(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "缺少code参数",
		})
		return
	}
	// code就是openid（从扫码状态查询接口获取）
	loginOrRegisterByWeChatId(code, c)
}

// wechatOffiAccountBindConfirm 新模式微信绑定确认
func wechatOffiAccountBindConfirm(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "缺少code参数",
		})
		return
	}

	if model.IsWeChatIdAlreadyTaken(code) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该微信账号已被绑定",
		})
		return
	}

	bindWeChatIdToCurrentUser(code, c)
}

// bindWeChatIdToCurrentUser 绑定WeChatId到当前登录用户
func bindWeChatIdToCurrentUser(wechatId string, c *gin.Context) {
	if model.IsWeChatIdAlreadyTaken(wechatId) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该微信账号已被绑定",
		})
		return
	}
	session := sessions.Default(c)
	id := session.Get("id")
	user := model.User{
		Id: id.(int),
	}
	err := user.FillUserById()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	user.WeChatId = wechatId
	err = user.Update(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
