## 全局公共参数
#### 全局Header参数
参数名 | 示例值 | 参数描述
--- | --- | ---
暂无参数
#### 全局Query参数
参数名 | 示例值 | 参数描述
--- | --- | ---
暂无参数
#### 全局Body参数
参数名 | 示例值 | 参数描述
--- | --- | ---
暂无参数
#### 全局认证方式
```text
noauth
```
#### 全局预执行脚本
```javascript
暂无预执行脚本
```
#### 全局后执行脚本
```javascript
暂无后执行脚本
```
## /AI接口
```text
认证使用在header中加入 Authorization:Bearer秘钥
```
#### Header参数
参数名 | 示例值 | 参数描述
--- | --- | ---
暂无参数
#### Query参数
参数名 | 示例值 | 参数描述
--- | --- | ---
暂无参数
#### Body参数
参数名 | 示例值 | 参数描述
--- | --- | ---
暂无参数
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
## /AI接口/视频生成
```text
# 模型id支持:
- doubao-seedance-1-0-pro-fast-251015
- doubao-seedance-1-5-pro-251215
- doubao-seedance-2-0-fast-260128
- doubao-seedance-2-0-260128

[参考文档](https://www.volcengine.com/docs/82379/1520757?lang=zh)
# content参数
输入给模型，生成视频的信息，支持文本、图片、音频、视频。支持以下几种组合：
- 文本
- 文本（可选）+ 图片
- 文本（可选）+ 视频
- 文本（可选）+ 图片 + 音频
- 文本（可选）+ 图片 + 视频
- 文本（可选）+ 视频 + 音频
- 文本（可选）+ 图片 + 视频 + 音频

### **注意**
- **图生视频-首帧、图生视频-首尾帧、多模态参考生视频（包括参考图、视频、音频）为 3 种互斥场景，不可混用。**
- 多模态参考生视频可通过提示词指定参考图片作为首帧/尾帧，间接实现“首尾帧+多模态参考”效果。若需严格保障首尾帧和指定图片一致，优先使用图生视频-首尾帧（配置 role 为 first_frame / last_frame）。

### 传入单张图片要求
- 格式：jpeg、png、webp、bmp、tiff、gif
- 宽高比（宽/高）： (0.4, 2.5) 
- 宽高长度（px）：(300, 6000)
- 大小：单张图片小于 30 MB。请求体大小不超过 64 MB。大文件请勿使用Base64编码。
- 图片数量：
  - 图生视频-首帧：1 张
  - 图生视频-首尾帧：2 张
  - Seedance 2.0 & 2.0 fast 多模态参考生视频：1~9 张
- image_url.url支持图片 URL 、图片 Base64 编码、素材 ID
  - 图片 URL：填入图片的公网 URL。
  - Base64 编码：将本地文件转换为 Base64 编码字符串，然后提交给大模型。遵循格式：data:image/<图片格式>;base64,<Base64编码>，注意 <图片格式> 需小写，如 data:image/png;base64,{base64_image}。
  - 素材 ID：用于视频生成的预置素材及虚拟人像的 ID，遵循格式：asset://<ASSET_ID> 
  - 素材ID使用格式eg:
  
js
{
"type": "image_url",
"image_url": {"url": "asset://asset-20260319082447-qrrjp"},
"role": "reference_image"
}
### 传入单个音频要求
- 输入给模型的音频信息。仅 Seedance 2.0 & 2.0 fast 支持输入音频。注意不可单独输入音频，应至少包含 1 个参考视频或图片。
- audio_url.url 支持URL 、音频 Base64 编码、素材 ID。
    - 音频 URL：填入音频的公网 URL。
    - Base64 编码：将本地文件转换为 Base64 编码字符串，然后提交给大模型。遵循格式：data:audio/<音频格式>;base64,<Base64编码>，注意 <音频格式> 需小写，如 data:audio/wav;base64,{base64_audio}。
    - 素材 ID：用于视频生成的虚拟人的音频素材 ID，遵循格式：asset://<ASSET_ID>。
- 格式：wav、mp3
- 时长：单个音频时长 [2, 15] s，最多传入 3 段参考音频，所有音频总时长不超过 15 s。
- 大小：单个音频不超过 15 MB，请求体大小不超过 64 MB。大文件请勿使用Base64编码。

### 传入单个视频要求
- video_url.url 支持视频URL、素材 ID。
    - 视频 URL：填入视频的公网 URL。
    - 素材 ID：用于视频生成的预置素材及虚拟人像视频的 ID，遵循格式：asset://<ASSET_ID>
- 视频格式：mp4、mov。
- 分辨率：480p、720p
- 时长：单个视频时长 [2, 15] s，最多传入 3 个参考视频，所有视频总时长不超过 15s。
- 尺寸：
  - 宽高比（宽/高）：[0.4, 2.5]
  - 宽高长度（px）：[300, 6000]
  - 画面像素（宽 × 高）：[409600, 927408] ，示例：
    - 画面尺寸 640×640=409600 满足最小值 ；
    - 画面尺寸 834×1112=927408 满足最大值。
- 大小：单个视频不超过 50 MB。
- 帧率 (FPS)：[24, 60] 

# resolution 参数

视频分辨率，取值范围：
- 480p
- 720p

# ratio 参数
生成视频的宽高比例。不同宽高比对应的宽高像素值见下方表格。
- 16:9 
- 4:3
- 1:1
- 3:4
- 9:16
- 21:9
- adaptive：根据输入自动选择最合适的宽高比
## adaptive 适配规则
- 当配置 ratio 为 adaptive 时，模型会根据生成场景自动适配宽高比；实际生成的视频宽高比可通过 查询视频生成任务 API 返回的 ratio 字段获取。
- 文生视频：根据输入的提示词，智能选择最合适的宽高比。
- 首帧 / 首尾帧生视频：根据上传的首帧图片比例，自动选择最接近的宽高比。
- 多模态参考生视频：根据用户提示词意图判断，如果是首帧生视频/编辑视频/延长视频，以该图片/视频为准选择最接近的宽高比；否则，以传入的第一个媒体文件为准（优先级：视频＞图片）选择最接近的宽高比。


![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869b92890cc886.png "image.png")

# token测算方式

![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869b944ce4cb98.png "image.png")
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/video/generations

#### 请求方式
> POST

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer sk-*** | String | 是 | -
#### 请求Body参数
```javascript
{
    "model": "doubao-seedance-2-0-fast-260128",
    "content": [
        {
            "type": "text",
            "text": "全程使用视频1的第一视角构图，全程使用音频1作为背景音乐。第一人称视角果茶宣传广告，seedance牌「苹苹安安」苹果果茶限定款；首帧为图片1，你的手摘下一颗带晨露的阿克苏红苹果，轻脆的苹果碰撞声；2-4 秒：快速切镜，你的手将苹果块投入雪克杯，加入冰块与茶底，用力摇晃，冰块碰撞声与摇晃声卡点轻快鼓点，背景音：「鲜切现摇」；4-6 秒：第一人称成品特写，分层果茶倒入透明杯，你的手轻挤奶盖在顶部铺展，在杯身贴上粉红包标，镜头拉近看奶盖与果茶的分层纹理；6-8 秒：第一人称手持举杯，你将图片2中的果茶举到镜头前（模拟递到观众面前的视角），杯身标签清晰可见，背景音「来一口鲜爽」，尾帧定格为图片3。背景声音统一为女生音色。"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "asset://asset-20260319082447-qrrjp"
            },
            "role": "reference_image"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/r2v_tea_pic1.jpg"
            },
            "role": "reference_image"
        },
        {
            "type": "image_url",
            "image_url": {
                "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/r2v_tea_pic2.jpg"
            },
            "role": "reference_image"
        },
        {
            "type": "video_url",
            "video_url": {
                "url": "https://ark-project.tos-cn-beijing.volces.com/doc_video/r2v_tea_video1.mp4"
            },
            "role": "reference_video"
        },
        {
            "type": "audio_url",
            "audio_url": {
                "url": "https://ark-project.tos-cn-beijing.volces.com/doc_audio/r2v_tea_audio1.mp3"
            },
            "role": "reference_audio"
        }
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 11,
    "watermark": false,
    "resolution": "720p"
}
```
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
model | doubao-seedance-2-0-fast-260128 | String | 是 | 模型名称 doubao-seedance-2-0-fast-260128或doubao-seedance-2-0-260128
content | - | Array | 是 | -
content.type | text | String | 是 | -
content.text | 全程使用视频1的第一视角构图，全程使用音频1作为背景音乐。第一人称视角果茶宣传广告，seedance牌「苹苹安安」苹果果茶限定款；首帧为图片1，你的手摘下一颗带晨露的阿克苏红苹果，轻脆的苹果碰撞声；2-4 秒：快速切镜，你的手将苹果块投入雪克杯，加入冰块与茶底，用力摇晃，冰块碰撞声与摇晃声卡点轻快鼓点，背景音：「鲜切现摇」；4-6 秒：第一人称成品特写，分层果茶倒入透明杯，你的手轻挤奶盖在顶部铺展，在杯身贴上粉红包标，镜头拉近看奶盖与果茶的分层纹理；6-8 秒：第一人称手持举杯，你将图片2中的果茶举到镜头前（模拟递到观众面前的视角），杯身标签清晰可见，背景音「来一口鲜爽」，尾帧定格为图片3。背景声音统一为女生音色。 | String | 是 | -
generate_audio | true | Boolean | 是 | 控制生成的视频是否包含与画面同步的声音。
ratio | 16:9 | String | 是 | 生成视频的宽高比例
duration | 11 | Integer | 是 | 生成视频时长，仅支持整数，单位：秒。
2.0模型取值范围：
- [4,15] 或设置为-1
watermark | false | Boolean | 是 | 水印
resolution | 720p | String | 是 | 视频分辨率Seedance 2.0 & 2.0 fast  默认值：720p
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"id": "cgt-20260317165706-4lpxk"
	}
}
```
## /AI接口/视频任务查询
```text
暂无描述
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/video/generations/tasks/cgt-202603******-rxpvn

#### 请求方式
> GET

#### Content-Type
> none

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer | String | 是 | -
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"id": "cgt-202603******-rxpvn",
		"model": "doubao-seedance-1-0-pro-fast-251015",
		"status": "succeeded",
		"content": {
"video_url": "https://ark-content-generation-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-1-0-pro-fast/example.mp4?X-Tos-Signature=REDACTED"
		},
		"usage": {
			"completion_tokens": 59160,
			"total_tokens": 59160
		},
		"created_at": 1773395884,
		"updated_at": 1773395903,
		"seed": 97660,
		"resolution": "1080p",
		"ratio": "16:9",
		"frames": 29,
		"framespersecond": 24,
		"service_tier": "default",
		"execution_expires_after": 172800,
		"draft": false
	}
}
```
## /AI接口/素材上传
```text
## **注意**
- 仅可使用已入库素材的 ID (Asset ID)进行视频生成，同一形象未入库素材无法使用。
- 仅需入库推理需使用的素材，不需使用的素材请勿入库。
- 目前已同步支持音频视频图片素材资产上传
## **单张图片要求**
- 格式：jpeg、png、webp、bmp、tiff、gif、heic/heif
- 宽高比（宽/高）： (0.4, 2.5) 
- 宽高长度（px）：(300, 6000)
- 大小：单张图片小于 30 MB。
## **传入单个视频要求**
- 格式：mp4、mov
- 分辨率：480p、720p
- 时长：单个视频时长 [2, 15] s
- 尺寸：
  - 宽高比（宽/高）：[0.4, 2.5]
  - 宽高长度（px）：[300, 6000]
  - 总像素数：[640×640=409600, 834×1112=927408]，即宽和高的乘积符合 [409600, 927408] 的区间要求。
- 大小：单个视频不超过 50 MB
- 帧率 (FPS)：[24, 60] 
## **传入单个音频要求**
- 格式：wav、mp3
- 时长：单个音频时长 [2, 15] s
- 大小：单个音频不超过 15 MB
# 警告：
**您需确保上传的虚拟人像符合以下条件：**
- 您合法拥有该素材，并享有完整的使用及处分权限。素材不包含未获授权的第三方商标、标识类内容。
- 素材不得与任何自然人肖像或形象雷同，素材不存在抄袭、盗用情形，不会侵害任何第三方的人格权、知识产权等合法权益。
- 素材不包含违反法规、违背公序良俗、危害国家安全的内容。



![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869c77d5668224.png "image.png")
eg.

![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869c77d8792410.png "image.png")
其中提示词如下:
js
背景参考图片1，月白虚影闪过，公子（妆造参考图片2；人物形象严格参考图片3）旋身开合折扇，鎏金扇刃弹出，墨竹扇面翻飞，慢鼓重响 1 声；特写：折扇刃格挡反派长刀，扇骨与刀身相击，公子唇角勾轻佻笑意，眼神却冷冽，指腹轻转扇柄。；慢镜：公子侧身贴地滑步，折扇刃贴反派腿侧划过，带起一道浅痕，锦袍下摆扫过地面，玉簪轻晃。快切：公子旋身抬手，折扇刃飞射而出，擦过反派脖颈，钉入身后木柱，反派僵立不敢动。反转：身后突然传来掌风，公子旋身接掌，指尖相触时借力后跳，折扇刃从木柱飞回手中，眼神警惕。慢镜高光：公子折扇半开，扇刃抵唇侧，抬眸望向身后，碎发被风吹起，眉梢微扬，带一丝桀骜。拉镜：公子立于庭院石台上，折扇轻摇，镜头拉远，庭院四角同时浮现戴面具的黑影（持弯刀，呈合围之势）。定格：公子折扇合起一半，扇刃露鎏金锋芒，抬步向前，画面压暗，只留他的侧影和扇刃光，音效骤停。音效：折扇开合脆响 + 刃风切割声 + 慢鼓卡点（偏沉稳）。


![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869c77decb8288.png "image.png")
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/asset/upload

#### 请求方式
> POST

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer | String | 是 | -
#### 请求Body参数
```javascript
{
    "image_url": "https://deep-market.tos-cn-beijing.volces.com/deepmarket/202603/69ba111259503.jpg",
    "asset_type":"Image"
}
```
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
image_url | https://deep-market.tos-cn-beijing.volces.com/deepmarket/202603/69ba111259503.jpg | String | 是 | 素材图片地址
asset_type | Image | String | 是 | 支持Image Video  Audio三种类型
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"ResponseMetadata": {
			"Action": "CreateAsset",
			"Region": "cn-beijing",
			"RequestId": "20260319162446B5DF7E4FBBC56F78E6DA",
			"Service": "ark",
			"Version": "2024-01-01"
		},
		"Result": {
			"Id": "asset-20260319082447-qrrjp"
		}
	}
}
```
## /AI接口/素材状态查询
```text
通过此接口获取素材是否生效，直到 Active / Failed / 超时
### 目前已知的Status状态枚举值有如下:
- Processing  → 继续轮询
- Active      → 返回 URL（结束）状态为 Active 后，可使用该素材 Asset ID (URI格式) 进行视频生成
- Failed      → 返回错误
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/asset/status

#### 请求方式
> POST

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer | String | 是 | -
#### 请求Body参数
```javascript
{"asset_id":"asset-20260319075230-7jqfr"}
```
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
asset_id | asset-20260319075230-7jqfr | String | 是 | 素材id
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"ResponseMetadata": {
			"Action": "GetAsset",
			"Region": "cn-beijing",
			"RequestId": "202603191630140FA78DC4D1524C75652C",
			"Service": "ark",
			"Version": "2024-01-01"
		},
		"Result": {
			"AssetType": "Image",
			"CreateTime": "2026-03-19T07:52:30Z",
			"GroupId": "group-20260319031427-tn957",
			"Id": "asset-20260319075230-7jqfr",
			"Name": "",
			"ProjectName": "default",
			"Status": "Active",
			"URL": "https://ark-media-asset.tos-cn-beijing.volces.com/2105684286/example.jpg?X-Tos-Signature=REDACTED",
			"UpdateTime": "2026-03-19T07:52:33Z"
		}
	}
}
```
参数名 | 示例值 | 参数类型 | 参数描述
--- | --- | --- | ---
code | 0 | Integer | -
msg | ok | String | -
data | - | Object | -
data.ResponseMetadata | - | Object | -
data.ResponseMetadata.Action | GetAsset | String | -
data.ResponseMetadata.Region | cn-beijing | String | -
data.ResponseMetadata.RequestId | 202603191630140FA78DC4D1524C75652C | String | -
data.ResponseMetadata.Service | ark | String | -
data.ResponseMetadata.Version | 2024-01-01 | String | -
data.Result | - | Object | -
data.Result.AssetType | Image | String | -
data.Result.CreateTime | 2026-03-19T07:52:30Z | String | -
data.Result.GroupId | group-20260319031427-tn957 | String | -
data.Result.Id | asset-20260319075230-7jqfr | String | -
data.Result.Name | - | String | -
data.Result.ProjectName | default | String | -
data.Result.Status | Active | String | Active为可用
data.Result.URL | https://ark-media-asset.tos-cn-beijing.volces.com/2105684286/example.jpg?X-Tos-Signature=REDACTED | String | -
data.Result.UpdateTime | 2026-03-19T07:52:33Z | String | -
## /AI接口/提交图片生成任务
```text
## 模型ID支持
- doubao-seedream-5-0-260128
- doubao-seedream-4-5-251128
- doubao-seedream-4-0-250828

不同模型特性以及参数参考文档https://www.volcengine.com/docs/82379/1541523?lang=zh
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/image/generations

#### 请求方式
> POST

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer sk-*** | String | 是 | -
#### 请求Body参数
```javascript
{
	"model": "doubao-seedream-5-0-260128",
	"prompt": "生成3张女孩和奶牛玩偶在游乐园开心地坐过山车的图片，涵盖早晨、中午、晚上",
	"image": [
		"https://ark-project.tos-cn-beijing.volces.com/doc_image/seedream4_imagesToimages_1.png",
		"https://ark-project.tos-cn-beijing.volces.com/doc_image/seedream4_imagesToimages_2.png"
	],
	"sequential_image_generation": "auto",
	"sequential_image_generation_options": {
		"max_images": 3
	},
	"size": "2K",
	"output_format": "png",
	"watermark": false
}
```
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
model | doubao-seedream-5-0-260128 | String | 是 | 模型名称
prompt | 生成3张猫 | String | 是 | -
image | https://ark-project.tos-cn-beijing.volces.com/doc_image/seedream4_imagesToimages_1.png | Array | 是 | -
sequential_image_generation | auto | String | 是 | -
sequential_image_generation_options | - | Object | 是 | -
sequential_image_generation_options.max_images | 1 | Integer | 是 | -
size | 2K | String | 是 | -
output_format | png | String | 是 | -
watermark | false | Boolean | 是 | 水印
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"data": {
			"id": "I20260401210457-4767-8dc442"
		}
	}
}
```
参数名 | 示例值 | 参数类型 | 参数描述
--- | --- | --- | ---
code | 0 | Integer | -
msg | ok | String | -
data | - | Object | -
data.data | - | Object | -
data.data.id | I20260401210457-4767-8dc442 | String | 任务id
## /AI接口/查询图片任务生成状态
```text
平均完成时间约30秒左右，如果超过20分钟还未完成任务，可能是与模型通讯异常，会被设置为failed
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/image/generations/tasks/I20260401205707-4573-d72831

#### 请求方式
> GET

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer sk-*** | String | 是 | -
#### 请求Body参数
```javascript

```
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"model": "doubao-seedream-5-0-260128",
		"created": 1775048736,
		"data": [
			{
		"url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedream-5-0/example.png?X-Tos-Signature=REDACTED",
				"size": "2048x2048"
			}
		],
		"usage": {
			"generated_images": 1,
			"output_tokens": 16384,
			"total_tokens": 16384
		},
		"status": "done"
	}
}
```
#### 错误响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"error": {
			"code": "InputTextSensitiveContentDetected",
			"message": "The request failed because the input text may contain sensitive information. Request id: 021775048228405beb22b84618335a214e9b1bc67d5e6c3ad7543",
			"param": "",
			"type": ""
		},
		"status": "failed"
	}
}
```
参数名 | 示例值 | 参数类型 | 参数描述
--- | --- | --- | ---
code | 0 | Integer | -
msg | ok | String | -
data | - | Object | -
data.error | - | Object | -
data.error.code | InputTextSensitiveContentDetected | String | -
data.error.message | The request failed because the input text may contain sensitive information. Request id: 021775048228405beb22b84618335a214e9b1bc67d5e6c3ad7543 | String | -
data.error.param | - | String | -
data.error.type | - | String | -
data.status | failed | String | -
## /AI接口/发起字幕擦除任务
```text
### 接口说明
使用 Seedance 2.0 / Seedance 2.0 fast 模型生成的视频可能包含字幕，使用本接口对视频进行字幕擦除任务提交与查询。

### 流程简介

**字幕擦除任务接口是异步接口，流程如下：**

1. 发起字幕擦除任务
2. 定时使用查询接口查询视频生成任务状态

    1.任务 running，过段时间再查询任务状态    
    2.任务 completed，返回视频URL，在24小时内下载生成的视频文件


当请求的参数或鉴权信息不正确时，任务将不会被创建，接口会返回一个同步的错误响应。详见[错误码](https://www.volcengine.com/docs/6448/2300662?lang=zh)。

![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869ddb9f05a587.png "image.png")

![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869ddb9cdc07c4.png "image.png")
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/ark-tools/ark-erase-video-subtitle-pro

#### 请求方式
> POST

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer sk-***** | String | 是 | -
#### 请求Body参数
```javascript
{
	"video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/example.mp4?X-Tos-Signature=REDACTED"
}
```
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
video_url | https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/example.mp4?X-Tos-Signature=REDACTED | String | 是 | 火山大模型生成视频源地址
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"success": true,
		"task_id": "amk-tool-ark-erase-video-subtitle-pro-92912724482",
		"request_id": "202604141312142B93E8DC664B76B9ED08"
	}
}
```
参数名 | 示例值 | 参数类型 | 参数描述
--- | --- | --- | ---
code | 0 | Integer | -
msg | ok | String | -
data | - | Object | -
data.success | true | Boolean | 任务是否提交成功。 
- true：成功。 
- false：失败。更多信息请查看错误处理。
data.task_id | amk-tool-ark-erase-video-subtitle-pro-92912724482 | String | 任务的唯一标识，用于后续查询任务进度和结果
data.request_id | 202604141312142B93E8DC664B76B9ED08 | String | 本次请求的唯一标识，可用于问题排查。
#### 错误响应示例
```javascript
{
	"code": 400,
	"msg": {
		"code": "AccessDenied",
		"type": "Forbidden",
		"message": "InvalidParameter.InvalidAPIKey:Get api key error from ark: Code:InvalidParameter.APIKey, Message:The specified parameter APIKey is invalid."
	}
}
```
## /AI接口/字幕擦除任务查询
```text
# 查询字幕擦除任务结果

![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869ddce245b217.png "image.png")

![image.png](https://img.cdn.apipost.cn/client/user/272223/avatar/78805a221a988e79ef3f42d7c5bfd41869ddce2e4fb62.png "image.png")
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/ark-tools/ark-tasks/{task_id}

#### 请求方式
> GET

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer sk-** | String | 是 | -
#### 路径变量
参数名 | 示例值 | 参数描述
--- | --- | ---
task_id | - | -
#### 请求Body参数
```javascript

```
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
#### 成功响应示例
```javascript
{
	"code": 0,
	"msg": "ok",
	"data": {
		"success": true,
		"task_id": "amk-tool-ark-erase-video-subtitle-pro-92912724482",
		"task_type": "ark-erase-video-subtitle-pro",
		"status": "completed",
		"result": {
			"video_url": "https://2124636113-amk-2105684286-default-041511.vod.cn-north-1.volcvideo.com/950cd9c1a4cf44f89955c5794d50baf2?preview=1&auth_key=1776230129-r0-u0-c2e85dfd8ad8c3fe88fa178db39622cb"
		},
		"expires_at": 1776230129,
		"created_at": 1776143537,
		"finished_at": 1776143729,
		"request_id": "20260414131623C0678BF62D35DA06967F"
	}
}
```
参数名 | 示例值 | 参数类型 | 参数描述
--- | --- | --- | ---
code | 0 | Integer | -
msg | ok | String | -
data | - | Object | -
data.success | true | Boolean | -
data.task_id | amk-tool-ark-erase-video-subtitle-pro-92912724482 | String | -
data.task_type | ark-erase-video-subtitle-pro | String | -
data.status | completed | String | -
data.result | - | Object | -
data.result.video_url | https://2124636113-amk-2105684286-default-041511.vod.cn-north-1.volcvideo.com/950cd9c1a4cf44f89955c5794d50baf2?preview=1&auth_key=1776230129-r0-u0-c2e85dfd8ad8c3fe88fa178db39622cb | String | 擦除字幕后的视频地址
data.expires_at | 1776230129 | Integer | -
data.created_at | 1776143537 | Integer | -
data.finished_at | 1776143729 | Integer | -
data.request_id | 20260414131623C0678BF62D35DA06967F | String | -
## /AI接口/对话(Chat) API
```text
对话API [参考文档](https://www.volcengine.com/docs/82379/1494384?lang=zh)
支持的模型id:
- doubao-seed-2-0-lite-260215
- doubao-seed-2-0-mini-260215
- doubao-seed-2-0-pro-260215
```
#### 接口状态
> 已完成

#### 接口URL
> https://api.pingxingshijie.cn/v2/chat/completions

#### 请求方式
> POST

#### Content-Type
> json

#### 请求Header参数
参数名 | 示例值 | 参数类型 | 是否必填 | 参数描述
--- | --- | --- | --- | ---
Authorization | Bearer sk-** | String | 是 | -
#### 请求Body参数
```javascript
{
    "model": "doubao-seed-2-0-lite-260215",
    "messages": [
        {
            "role": "user",
            "content": "你好"
        }
    ],
    "stream": true
}
```
#### 认证方式
```text
noauth
```
#### 预执行脚本
```javascript
暂无预执行脚本
```
#### 后执行脚本
```javascript
暂无后执行脚本
```
