# AIChatBot -- AI对话机器人

AIChatBot使用微信测试号对接任何兼容OpenAI规范的API接口,（例如：[**AIGC2D**](https://www.aigc2d.com)
）,实现对话助手，已添加Google联网搜索支持

![截图](/image/example.jpg)

## 安装和配置

### 准备资料

1. [访问并开通微信测试号](https://mp.weixin.qq.com/debug/cgi-bin/sandbox?t=sandbox/login)

将页面上的 appID、appsecret作为环境变量

```yaml
- MP_APPID=appID
- MP_APPSECRET=appsecret
```

2. 随机编写一个Token

```yaml
- MP_TOKEN=your-random-token
```

3. 配置OpenAI接口

OpenAI 配置示例

```yaml
- DEFAULT_API_URL=https://api.openai.com/v1
- DEFAULT_API_KEY=sk...
- DEFAULT_MODEL=gpt-3.5-turbo-16k
```

AIGC2D 配置示例

```yaml
- DEFAULT_API_URL=https://api.aigc2d.com/v1
- DEFAULT_KEY=aigc2d...
- DEFAULT_MODEL=gpt-3.5-turbo-16k
```

4. 设置触发词
   可通过以下环境变量设置触发词，设置后只有包含触发词的对话才会触发AI答复。如果测试为空，那么每一句都可以触发AI答复。

```yaml
- DEFAULT_WORD
```

5. 配置Google搜索引擎插件
   Google搜索引擎插件是借助 [**serper**](https://serper.dev/) 实现，对比市面上的Google搜索价格应该是最便宜的，注册赠送2500次请求

```yaml
- ENABLE_SEARCH=true
- SERPER_KEY=xxxx
```

6. 启动服务后（见后），记得将回调地址和Token回填到微信测试号设置页面。

### Docker

> 请参考[环境变量说明](#环境变量说明)修改环境变量

> 确保对 ./data 有写权限，或者挂到有权限的地方去,如果不需要持久化保存数据则可以不选择这个

```shell
docker run -d --name bot \
-e MP_APPID= \
-e MP_SECRET= \
-e MP_TOKEN= \
-e DEFAULT_API_URL=https://api.agic2d.com/v1 \
-e DEFAULT_WORD= \  
-e DEFAULT_KEY=aigc2d... \
-v ./data:/app/data \
-p 80:8080 \
alone88/aichatbot:latest

```

### docker-compose

> 请参考[环境变量说明](#环境变量说明)修改环境变量

> 确保对 ./data 有写权限，或者挂到有权限的地方去,如果不需要持久化保存数据则可以不选择这个
 
> 复制 docker-compose.example.yml 为 docker-compose.yml，并根据需要修改环境变量：

```shell
version: '3'
services:
  bot:
    image: alone88/aichatbot
    environment:
      - MP_APPID=
      - MP_SECRET=
      - MP_TOKEN=
      - DEFAULT_API_URL=https://api.aigc2d.com/v1
      - DEFAULT_WORD=
      - DEFAULT_API_KEY=aigc2d...
      - ENABLE_HISTORY=true
    volumes:
      - ./data:/app/data
    ports:
      - 80:8080
```

启动服务

```shell
docker-compose up -d
```


## 回填Token和回调地址

启动服务后，我们可以获得`回调地址`。如果IP/域名是`xxx.xxx.xxx.xxx:xxxx`，那么`回调地址`是`http://xxx.xxx.xxx.xxx:xxxx/wechat`

> 如果你在本机测试，需要使用 ngrok 等内网穿透工具创建一个公网可以访问的域名

回到测试号设置页面，点击 `接口配置信息` 后的修改链接，会将 `回调地址` 和 `TOKEN`( 即 `MP_TOKEN` )  填入并保存。

![](/image/wechat_example.png)


## 使用

用微信扫描测试号设置页面的二维码，关注测试号以后，可以发送问题。也可以通过 `/setXXX` 命令进行针对个人的配置。

可以通过 `/help` 查看可用命令。

> 你可以将测试号「发送到桌面」，作为快速进入的入口。这样就不用在微信里边到处找了 (好像只有安卓才支持)


### 环境变量说明

| 变量名                | 说明                                              |
|--------------------|-------------------------------------------------|
| MP_APPID           | 微信公众号APPID                                      |
| MP_SECRET          | 微信公众号Secret                                     |
| MP_TOKEN           | 微信公众号Token                                      |
| STREAM             | 是否采用流式传输                                        |
| DEFAULT_KEY        | OpenAI/AIGC2D的apikey                            |
| DEFAULT_API_URL    | OpenAI/AIGC2D的接口地址，默认为https://api.aigc2d.com/v1 |
| DEFAULT_WORD       | 触发词，包含触发词才会触发回复,不设置则所有都会触发回复                    |
| DEFAULT_MODEL      | 模型名称，默认为gpt-3.5-turbo-16k                       |
| DEFAULT_SYSTEM     | 系统提示词，默认为空                                      |
| MAX_TOKEN          | 最大max_tokens限制，默认根据模型限定                         |
| TEMPERATURE        | 模型的temperature                                  |
| REPLY_CHUNK_LENGTH | 每次输出字数限制，超过这个限制则会分多条消息返回                        |
| ENABLE_HISTORY     | 是否保留对话上下文                                       |
| ENABLE_SEARCH      | 是否启用搜索引擎插件                                      |
| SERPER_KEY         | 搜索引擎的apikey                                     |

