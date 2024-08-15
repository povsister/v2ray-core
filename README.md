> This is a modified V2Ray-core maintained by myself.
>
> Updates from the upstream will be merged periodically.

<h3>It has several added features:</h3>

* OSPFv2 support when act as an on-demand transparent proxy.
* DNS-route capability.
* Conn-track capability for routing decision.
* HTTP health-check inbounds.

If you have any questions which are not related to features described above,
please submit it to [upstream project](https://github.com/v2fly/v2ray-core).

<div>
  <img width="190" height="210" align="left" src="https://raw.githubusercontent.com/v2fly/v2fly-github-io/master/docs/.vuepress/public/readme-logo.png" alt="V2Ray"/>
  <br>
  <h1>Project V</h1>
  <p>Project V is a set of network tools that helps you to build your own computer network. It secures your network connections and thus protects your privacy.</p>
</div>

[![GitHub Test Badge](https://github.com/povsister/v2ray-core/workflows/Test/badge.svg)](https://github.com/povsister/v2ray-core/actions)
[![codecov.io](https://codecov.io/gh/v2fly/v2ray-core/branch/master/graph/badge.svg?branch=master)](https://codecov.io/gh/povsister/v2ray-core?branch=master)
[![codebeat](https://goreportcard.com/badge/github.com/v2fly/v2ray-core)](https://goreportcard.com/report/github.com/povsister/v2ray-core)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/e150b7ede2114388921943bf23d95161)](https://www.codacy.com/gh/povsister/v2ray-core/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=povsister/v2ray-core&amp;utm_campaign=Badge_Grade)
[![Downloads](https://img.shields.io/github/downloads/v2fly/v2ray-core/total.svg)](https://github.com/povsister/v2ray-core/releases/latest)
<!-- TOC -->
* [Related Links](#related-links)
* [为什么开发本项目](#为什么开发本项目)
  * [方案差异对比](#方案差异对比)
  * [原理解释](#原理解释)
* [前置要求](#前置要求)
  * [理论知识](#理论知识)
  * [硬件要求](#硬件要求)
* [使用说明](#使用说明)
  * [0x1: 网络拓扑配置](#0x1-网络拓扑配置)
  * [0x2: 旁路由（V2Ray）配置](#0x2-旁路由v2ray配置)
    * [安装V2Ray并配置透明代理](#安装v2ray并配置透明代理)
      * [赋予V2Ray额外权限，用于支持OSPF协议](#赋予v2ray额外权限用于支持ospf协议)
    * [配置V2Ray的OSPF模块](#配置v2ray的ospf模块)
    * [配置IP masquerade](#配置ip-masquerade)
    * [收尾工作，开机自启，配置持久化](#收尾工作开机自启配置持久化)
      * [持久化开启内核IPv4转发参数](#持久化开启内核ipv4转发参数)
      * [V2Ray开机自启动](#v2ray开机自启动)
      * [将nftables的配置持久化](#将nftables的配置持久化)
  * [0x3: 主路由（ROS）配置](#0x3-主路由ros配置)
    * [开启OSPF动态路由协议](#开启ospf动态路由协议)
    * [配置策略路由以避免路由环路](#配置策略路由以避免路由环路)
    * [配置DNS转发旁路由](#配置dns转发旁路由)
    * [配置探活和探活失败时自动回切DNS的脚本](#配置探活和探活失败时自动回切dns的脚本)
* [V2Ray配置示例](#v2ray配置示例)
    * [更新记录](#更新记录)
    * [V2Ray监控预览](#v2ray监控预览)
    * [GFW黑名单+自定义黑名单配置示例](#gfw黑名单自定义黑名单配置示例)
* [FAQs](#faqs)
  * [OSPF 收敛速度快吗？](#ospf-收敛速度快吗)
  * [这个修改版的V2Ray为什么关闭有点慢](#这个修改版的v2ray为什么关闭有点慢)
<!-- TOC -->

# Related Links

- [Documentation](https://www.v2fly.org) and [Newcomer's Instructions](https://www.v2fly.org/guide/start.html)
- Welcome to translate V2Ray documents via [Transifex](https://www.transifex.com/v2fly/public/)

# 为什么开发本项目

先叠个甲，本方案配置较为繁琐，有硬件要求，且深度涉及计算机网络原理，仅适用于有进阶网络知识的用户使用。

本项目旨在解决：使用透明代理进行网关科学的模式下，网关直接使用软路由带来的稳定性问题，以及性能问题。

如果你不在意：默认使用软路由作为你的家庭网关，且可以接受折腾软路由时造成全部网络中断/抖动的问题，则本文方案可能不适合你。

**核心理念为：仅需要科学的流量会被转发至软路由处理，其余流量由主路由直接发出。
主路由使用常规硬路由以保证性能和稳定性。**

因此，对于旁路由的性能要求降到了一个非常低的水平，同时，软路由的任何故障对于网络的影响也基本消除。

类似的，以“按需转发流量”作为核心理念的方案有：[FakeDNS](https://www.v2fly.org/config/fakedns.html)。
我也试用过相当长一段时间，但其存在几个我无法接受的问题：

* FakeIP污染，例如：大陆白名单时，默认污染其他所有域名
* 旁路由故障/修改配置重启时，FakeIP污染会持续一段时间无法立即清除
* 需要手动在主路由上维护静态路由条目
* 无法灵活应对Telegram这种不使用系统DNS的软件
* 旁路由入侵网络拓扑，无法快速移除

相比之下，本方案具有以下优点：

* 全真IP，不存在FakeIP污染，同时解决国内环境的DNS污染
* 支持基于规则文件的IP路由规则，灵活应对Telegram类似的软件
* 分流黑白名单模式可按喜好配置，无任何副作用
* 默认支持 srcIP -> dstIP 作为pattern的Connection-track，路由决策无需Sniffing
* 旁路由可插拔，生效路由条目由旁路由自动通告，无需维护静态路由条目，网络拓扑可自动容灾
* 整体方案扩展能力强，可结合硬路由和软路由的各自特点，并充分利用各自的优势

## 方案差异对比

| 特性    | 软路由           | 硬路由           | 本方案（按需旁路）                 |
|-------|---------------|---------------|---------------------------|
| 科学能力  | 强，取决于ROM      | 弱，配置复杂+不灵活    | 强，包含所有V2Ray功能             |
| 性能    | 取决于硬件配置       | 远强于同规格软路由     | 强，直连性能等同于硬路由，科学性能取决于软路由配置 |
| 功耗    | 高             | 低             | 较低                        |
| NAT情况 | 取决于软件承诺       | 一般为FullClone  | 直连流量与硬路由无异，科学流量取决于V2Ray承诺 |
| 容灾    | 无，全部断网        | 配置复杂          | 自动恢复拓扑，科学流量可降级为直连         |
| 稳定性   | 低，重启/故障影响全部网络 | 高，仅受不可抗力影响    | 高，旁路由故障/重启不影响主干网络         |
| 扩展能力  | 低下限高上限，取决于ROM | 高下限，支持各种电信级玩法 | 高，可充分利用软硬路由各自优势           |

## 原理解释

下面的拓扑图表示了本项目中旁路由的工作方式。

图示过程描述了一台内网设备是如何在

* 不修改默认网关
* 不修改默认DNS
* 不安装代理软件

的情况下，无感知的通过网关透明代理，科学访问www.google.com的。

粉色的箭头表示DNS请求流程，绿色的箭头表示真正的访问流程（即实际传输数据的TCP/UDP过程）。

简单来说，主路由会把所有来自LAN的DNS请求，通过防火墙的DNAT规则转发给旁路由处理，
**旁路由会使用DNS请求的域名+DNS解析结果，预先进行一次路由决策**：

若域名+DNS解析后的IP

* 匹配代理出口的Tag：通过OSPFv2动态路由协议，**向主路由通告目标IP的下一跳为旁路由**，返回DNS解析结果，同时添加基于 srcIP ->
  dstIP 的conn-track规则
* 不匹配代理Tag：直接返回DNS解析结果即可

这一过程被我称作DNS Route：通过分析来自客户端的DNS请求，按需产生一条通往目标域名IP的路由规则。
同时，得益于动态添加的，基于源-目标IP的conn-track规则，后续连接出口的匹配可以直接跳过V2Ray的Sniffing，在ECH普及的未来仍可做到精准域名分流。

而且，由于科学访问的路由表由OSPF动态路由协议维护，旁路探活失败时主路由会自动恢复网络拓扑，
配合探活脚本自动回切防火墙的DNS转发规则，则可以完美的消除旁路故障对于主干网络的影响。

目前实现中，DNS Route的掩码为/32，有效时间为6个小时，6小时内没有任何DNS请求或者实际流量，则会自动废弃对应路由条目。
实际使用中，生效路由条目约为400-600条，配合fastTrack，对于主路由的性能影响可以忽略不计。

![How it works](/images/howItWorks.png)

# 前置要求

本项目中，V2Ray将被配置为旁路由透明代理使用，需要你事先掌握/具备以下条件：

## 理论知识

* 理解什么是透明代理
* 如何配置V2Ray以透明代理模式工作
* 理解单臂路由（旁路由）的基本工作原理
* 理解路由设备的工作原理，熟悉路由决策过程，理解路由表及防火墙基本原理
* 熟悉nftables，具备基本的linux操作能力

## 硬件要求

* 支持OSPFv2动态路由协议的主路由，且主路由需要支持策略路由（某些文章可能称为标记路由）。
* 一台可运行V2Ray的Debian Linux作为旁路由

**以下的使用说明中，采用的硬件配置为**

主路由：MikroTik hAPac2 RBD52G-5HacD2HnD (RouterOS v6.49.14)

旁路由：Debian 11 Linux with 2-core 2GiB RAM (ESXi 7.0 on J4125)

推荐主路由使用ROS，旁路由使用Debian11及以上的linux系统，至少分配1c1g的资源。

# 使用说明

## 0x1: 网络拓扑配置

请参考如下拓扑，配置好主路由与旁路由。

**核心诉求只有两点：**

* 主路由与旁路由需要和LAN设备隔离出一个网段，且这个网段只有主路由和旁路由两个设备，这个是必须要求。
* 主路由与旁路由IP固定

![Network topology](/images/topology.png)

## 0x2: 旁路由（V2Ray）配置

简单来说，旁路由配置主要有以下步骤：配置透明代理，配置OSPF相关参数/健康检查端口，配置IP masquerade等。

以下所有命令中 `${IFNAME}` 均代表软路由和主路由连接的网卡名称，可以使用 `ifconfig` `ip link` 等命令查看。

使用时需要替换成你自己环境中的网卡名称。

### 安装V2Ray并配置透明代理

linux系统上推荐使用 [fhs-install-v2ray](https://github.com/v2fly/fhs-install-v2ray)
下载并安装V2Ray

然后下载[本项目Release页](https://github.com/povsister/v2ray-core/releases)
里的修改版，替换v2ray可执行文件即可，v2ray的默认安装路径为 `/usr/local/bin/v2ray`

[透明代理的配置教程](https://guide.v2fly.org/app/tproxy.html)
已经很多，我就不再赘述了，核心要求只有以下几个：

* 只支持TPORXY模式的透明代理，请勿配置成REDIRECT模式。
* 需要拦截UDP53的DNS查询请求，并转交给V2Ray内置DNS处理
* 强烈推荐替换V2Ray的默认geoip规则文件为社区增强版本的 [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip)
* 需要参考本节末尾，**额外赋予 V2Ray `NET_RAW` 权限**，否则无法正常收发OSPF数据包

另外，[建议按照教程要求，修改v2ray的最大文件描述符限制](https://guide.v2fly.org/app/tproxy.html#%E8%A7%A3%E5%86%B3-too-many-open-files-%E9%97%AE%E9%A2%98)，
避免在处理UDP流量时出现问题。

#### 赋予V2Ray额外权限，用于支持OSPF协议

在 `/etc/systemd/system/v2ray.service.d/11-extra-capability.conf` 里创建以下内容

```shell
[Service]
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_NET_RAW
```

保存并退出，执行 `systemctl daemon-reload` 以便配置生效

### 配置V2Ray的OSPF模块

此模块为本项目完全独立开发的部分，得益于V2Ray良好的模块化设计，最终以`dnsCircuit`模块形式嵌入了V2Ray中，需要在配置文件中写入指定配置方可开启。

V2ray的默认配置文件路径为 `/usr/local/etc/v2ray/config.json`

不开启此模块时，此修改版的V2Ray与官方版本无异。

配置文件示例（节选），供参考。

以下内容核心有两块必填，其余配置请按自己实际情况填写。

* `dnsCircuit` 部分，**是本项目核心的功能模块**，用于启用DNS Route，并配置要监听的inbounds/outbounds等等
* `inbounds` 中需要配置一个HTTP健康检查入口，用于主路由检查旁路软件健康状况，这块后续主路由配置部分会再提到

```json5
{
  // 最重要的部分
  // 用于DNS route的配置
  "dnsCircuit": {
    //（必填）DNS outbound 的Tag，用于分析DNS请求并预决策路由。
    "dnsOutboundTag": "dns-out",
    //（必填）用于conn-track的inbound，目前只支持dokodemo-door协议的inbound。
    // 填写透明代理的inboundTag即可
    "inboundTags": [
      "transparent"
    ],
    //（outboundTags和balancerTags不能同时为空）
    // 用于conn-track的outbound，同时，DNS路由结果命中此outboundTag的流量都会被转发至旁路由。
    // 填写代理服务器的outboundTag即可
    "outboundTags": [
      "proxy"
    ],
    //（outboundTags和balancerTags不能同时为空）
    // 用于conn-track的balancer，同时，DNS路由结果命中此balancerTag的流量都会被转发至旁路由。
    "balancerTags": [
      "jp-balancer"
    ],
    //（可选）固定通告某些IP段，目标IP在此范围内的流量都会被转发至旁路由。
    "persistentRoute": [
      // 从规则文件中载入电报的服务器IP段，从而实现内网设备自动通过旁路由访问电报。
      "geoip:telegram",
      // 也可以直接以CIDR形式书写要转发至旁路的IP段，这块只是示例，请按自己实际情况填写。
      "10.0.0.0/8"
    ],
    //（可选）不活跃路由的清理时间（秒），不活跃时间超过这个数值后，对应路由条目和conn-track规则会被删除。
    // 默认：21600秒（6个小时）
    "inactiveClean": 21600,
    //（必填）OSPF设置
    // 需要填写软路由和主路由相连的网卡名称，以及软路由自己的IP和网段的掩码，以CIDR形式填写。
    "ospfSetting": {
      //（必填）软路由上的网卡名称
      "ifName": "ens160",
      //（必填）软路由自己的IP+子网掩码
      "address": "192.168.87.2/24"
    }
  },
  "inbounds": [
    {
      //（必填）用作代理软件健康检查
      "tag": "health-check",
      // 必须填写 0.0.0.0 否则无法接受来自旁路由的请求
      "listen": "0.0.0.0",
      // 端口可随意填写，注意和后面主路由配置的健康检查端口对应即可
      "port": 54321,
      //（必填）注意protocol一定要填写http-healthcheck
      "protocol": "http-healthcheck",
      "settings": {
        "timeout": 3
      }
    },
    {
      //（必填）透明代理 inbound，本方案中必须填写
      "tag": "transparent",
      "listen": "127.0.0.1",
      "port": 12345,
      "protocol": "dokodemo-door",
      "settings": {
        "network": "tcp,udp",
        "followRedirect": true
      },
      "sniffing": {
        // 本方案无需开启嗅探
        "enabled": false,
      },
      "streamSettings": {
        "sockopt": {
          // 透明代理必须使用 TPROXY 方式
          "tproxy": "tproxy",
          "mark": 255
        }
      }
    }
    // ... 省略不相干inbounds
  ],
  "outbounds": [
    {
      // 直连流量
      "tag": "direct",
      "protocol": "freedom",
      "settings": {
        "domainStrategy": "UseIPv4"
      },
      "streamSettings": {
        "sockopt": {
          "mark": 255
        }
      }
    },
    {
      // 代理出口
      "tag": "proxy",
      "protocol": "vmess",
      "settings": {
        "vnext": [
          {
            "address": "your.proxy.server",
            "port": 65535,
            "users": [
              {
                "id": "************************",
                "security": "auto"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "sockopt": {
          "mark": 255
        }
      }
    },
    {
      //（必填）dns outbound，用于接受DNS请求
      "tag": "dns-out",
      "protocol": "dns",
      "streamSettings": {
        "sockopt": {
          "mark": 255
        }
      }
    }
    // ... 省略不相干outbounds
  ],
  // 此处示例路由为GFW黑名单模式
  "routing": {
    // 建议使用此规则
    "domainStrategy": "IPIfNonMatch",
    "domainMatcher": "mph",
    "rules": [
      {
        // 直连 123 端口 UDP 流量（NTP 协议）
        "type": "field",
        "inboundTag": "transparent",
        "port": 123,
        "network": "udp",
        "outboundTag": "direct"
      },
      {
        // 劫持 53 端口 UDP 流量，使用 V2Ray 的 DNS
        "type": "field",
        "inboundTag": "transparent",
        "port": 53,
        "network": "udp",
        "outboundTag": "dns-out"
      },
      {
        // 直连 国内网站
        "type": "field",
        "domain": [
          "domain:ntp.org",
          "geosite:china-list",
          "geosite:cn",
          "geosite:tld-cn",
          "geosite:apple",
          "geosite:apple-cn",
          "geosite:google-cn",
          "geosite:icloud",
          "geosite:category-games@cn",
          // steam下载走国内CDN
          "domain:steamserver.net",
          "geosite:geolocation-cn"
        ],
        "outboundTag": "direct"
      },
      {
        // 直连 国内IP
        "type": "field",
        "ip": [
          "geoip:cn"
        ],
        "outboundTag": "direct"
      },
      {
        // telegram IP 走代理
        // 用于和dnsCircuit 的 persistentRoute 相配合
        "type": "field",
        "ip": [
          "geoip:telegram"
        ],
        "outboundTag": "proxy"
      },
      {
        // 墙的域名走代理
        "type": "field",
        "domain": [
          "geosite:gfw",
          "geosite:geolocation-!cn"
        ],
        "outboundTag": "proxy"
      },
      {
        //（重要，必填）
        // 注意顺序，建议紧跟在域名路由规则之后。
        // DNS Route 动态维护的 conn-track 规则，实际使用的是V2Ray router的 srcIP - dstIP 匹配规则。
        // 格式为: 
        // from: dynamic-ipset:dnscircuit-conntrack-src-{outboundTag}
        // to: dynamic-ipset:dnscircuit-conntrack-dest-{outboundTag}
        "type": "field",
        "source": "dynamic-ipset:dnscircuit-conntrack-src-proxy",
        "ip": "dynamic-ipset:dnscircuit-conntrack-dest-proxy",
        "outboundTag": "proxy"
      },
      {
        //（重要，必填）
        // 注意顺序，建议写在所有路由规则最后。
        // DNS Route 路由默认出口，当一个incoming连接没有被conn-track规则命中时，会被此规则兜底。
        // 照着写即可
        "type": "field",
        "ip": "dynamic-ipset:dnscircuit-dest-default",
        "outboundTag": "proxy"
      }
      // ... 其他路由规则省略
    ]
  },
  "dns": {
    // DNS应该使用国内外DNS分流的配置。
    // 此处暂时省略，后续完整配置示例会给出。
  }
}
```

### 配置IP masquerade

此配置的主要目的是，配合主路由上的策略路由规则，直接转发透明代理不处理的IP数据包（即，TCP/UDP协议以外的IP报文），
以及直接送出旁路由本身发出的流量，避免形成路由环路。

直接按要求设置即可，主路由配置时会再提到这部分。

运行命令，开启内核的IPv4包转发功能，并设置从旁路由网卡发出的IP包做masquerade，注意替换`IFNAME`为你自己的网卡名称。

```shell
# 开启内核IPv4包转发
sysctl -w net.ipv4.ip_forward=1

# 在POSTROUTING上添加一个名为v2ray的链
nft add chain v2ray postrouting { type nat hook postrouting priority 0 \; }
# 向v2ray链中添加一条规则，从 ${IFNAME} 网卡发出的流量全部进行masquerade
nft add rule v2ray postrouting oif ${IFNAME} masquerade
```

### 收尾工作，开机自启，配置持久化

主要是内核参数，透明代理策略和nftables的规则持久化。

#### 持久化开启内核IPv4转发参数

编辑 `/etc/sysctl.conf`，添加`net.ipv4.ip_forward=1`，
执行命令

```shell
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
```

#### V2Ray开机自启动

执行 `systemctl enable v2ray` 即可

然后用 `systemctl status v2ray` 确认设置，有出现enabled字样即可

```shell
root@debian:~# systemctl status v2ray
● v2ray.service - V2Ray Service
     Loaded: loaded (/etc/systemd/system/v2ray.service; enabled; vendor preset: enabled)
    Drop-In: /etc/systemd/system/v2ray.service.d
             └─10-donot_touch_single_conf.conf, 11-extra-capability.conf, 20-ulimit.conf
     Active: active (running) since Thu 2024-05-09 23:16:40 HKT; 17h ago
       Docs: https://www.v2fly.org/
   Main PID: 195546 (v2ray)
      Tasks: 9 (limit: 2337)
     Memory: 215.3M
        CPU: 7min 8.496s
     CGroup: /system.slice/v2ray.service
             └─195546 /usr/local/bin/v2ray run -config /usr/local/etc/v2ray/config.json

May 10 17:07:05 debian v2ray[195546]: 2024/05/10 17:07:05 192.168.88.192:44383 accepted udp:192.168.87.2:53 [dns-out]
```

#### 将nftables的配置持久化

首先，检查nftables配置，运行命令 `nft list ruleset`

你的配置应该和下面的输出类似，注意不要照抄。按自己实际情况确认。

```shell
root@debian:~# nft list ruleset
table inet filter {
	chain input {
		type filter hook input priority filter; policy accept;
	}

	chain forward {
		type filter hook forward priority filter; policy accept;
	}

	chain output {
		type filter hook output priority filter; policy accept;
	}
}
table ip v2ray {
	chain prerouting {
		type filter hook prerouting priority filter; policy accept;
		ip daddr { 127.0.0.1, 224.0.0.0/4, 255.255.255.255 } return
		meta l4proto tcp ip daddr 192.168.0.0/16 return
		ip daddr 192.168.0.0/16 udp dport != 53 return
		meta mark 0x000000ff return
		meta l4proto { tcp, udp } meta mark set 0x00000001 tproxy to 127.0.0.1:12345 accept
	}

	chain output {
		type route hook output priority filter; policy accept;
		ip daddr { 127.0.0.1, 224.0.0.0/4, 255.255.255.255 } return
		meta l4proto tcp ip daddr 192.168.0.0/16 return
		ip daddr 192.168.0.0/16 udp dport != 53 return
		meta mark 0x000000ff return
		meta l4proto { tcp, udp } meta mark set 0x00000001 accept
	}

	chain postrouting {
		type nat hook postrouting priority filter; policy accept;
		oif "ens160" masquerade
	}
}
table ip filter {
	chain divert {
		type filter hook prerouting priority mangle; policy accept;
		meta l4proto tcp socket transparent 1 meta mark set 0x00000001 accept
	}
}
```

确认无误后，保存规则至 `/etc/nftables/rules.v4`，需要执行以下命令

`nft list ruleset > /etc/nftables/rules.v4`

然后，新建systemd service，在 `/etc/systemd/system/tproxy.service` 创建以下内容，
目的是通过systemd管理自启任务。

```shell
[Unit]
Description=Tproxy rule
After=network.target
Wants=network.target

[Service]

Type=oneshot
RemainAfterExit=yes
ExecStart=/sbin/ip rule add fwmark 1 table 100 ; /sbin/ip route add local default dev lo table 100 ; /sbin/nft -f /etc/nftables/rules.v4
ExecStop=/sbin/ip rule del fwmark 1 table 100 ; /sbin/ip route del local default dev lo table 100 ; /sbin/nft flush ruleset

[Install]
WantedBy=multi-user.target
```

设置开机自启动，执行 `systemctl enable tproxy`即可。

设置完成后，可以用 `systemctl status tproxy` 确认有enabled字样即可

```shell
root@debian:~# systemctl status tproxy
● tproxy.service - Tproxy rule
     Loaded: loaded (/etc/systemd/system/tproxy.service; enabled; vendor preset: enabled)
     Active: active (exited) since Mon 2023-10-09 22:10:30 HKT; 7 months 0 days ago
   Main PID: 714 (code=exited, status=0/SUCCESS)
      Tasks: 0 (limit: 2337)
     Memory: 0B
        CPU: 0
     CGroup: /system.slice/tproxy.service
```

## 0x3: 主路由（ROS）配置

主路由配置基本是四块：开启OSPF动态路由协议，防止路由环路，DNS转发，以及旁路由探活和探活失败时自动回切DNS的脚本。

**主路由怎么配置正常上网我就不赘述了，本文默认你已经会使用ROS配置PPPoE拨号或者直接DHCP上网。**

### 开启OSPF动态路由协议

进入 `Routing -> OSPF` 菜单，如果是v7的ROS系统，记得选OSPFv2，本项目目前只支持IPv4

进入 `Interfaces`，选择和旁路由直接相连的接口，我这里旁路由和主路由接口都属于一个网桥，所以直接选网桥即可，如果你没用网桥，那就选接口。
验证选None，不开启验证，优先级填1，其他默认即可，务必保证`HelloInterval=10` 且 `RouterDeadInterval=40`，否则会影响邻接。
![ospf interfaces](/images/ospf-interfaces.png)

进入 `Instances`，填写主路由的RouterID，这里直接写主路由相对于旁路由网段的IP地址即可，例如在我的拓扑中，这里填写主路由IP `192.168.87.1`。
其他全默认即可，见下图
![ospf instances](/images/ospf-instances.png)

进入 `Network`，填写主路由和旁路由所属的网段以及掩码，Area选择默认的backbone即可，如下图所示
![ospf networks](/images/ospf-networks.png)

至此完成OSPF配置，等待40秒后，你的主路由 `Interface - State` 应该和上图一样，展示为 Designated Router（即DR）状态。

### 配置策略路由以避免路由环路

因为透明代理只能处理TCP和UDP流量，其他类型的IP数据包会由linux内核直接转发，
而主路由上的OSPF动态路由表，会无条件将所有OSPF通告目标IP的数据报文下一跳给旁路，旁路由的默认网关又是主路由。
因此，在极少数情况下，这个互相甩锅的过程，会造成路由环路的问题。

当然，V2Ray配置有误也会导致环路，这个暂且按下不表。

为了避免环路，需要识别出旁路由发出的流量，跳过OSPF的动态路由规则进行匹配。

这块就需要旁路由转发IP报文时，无条件做IP masquerade，然后，用主路由的策略路由功能进行分流，具体步骤为：

* **主路由创建一个新的路由表**，记为`side-anti-loop`，此路由表中需要填写默认路由为WAN口，以及本地LAN IP段所属的网桥或接口。

  注意红框中的内容，如果你本地有其他网段，需要一并以静态路由形式填入，注意选择所属接口。这块照图自己写吧，就不给命令了。
  ![New RoutingTable: side-anti-loop](/images/side-rtable.png)


* **主路由创建策略路由规则**：来自于旁路由IP 192.168.87.2的数据包，仅查询路由表`side-anti-loop`

  对应命令如下，其中`side-router`是我旁路由所在的网桥，`192.168.87.2`是我的旁路由IP，你可以视情况改成接口/你自己的旁路由IP。不要照抄。
  ```shell
  /ip route rule add src-address=192.168.87.2 interface=side-router action=lookup-only-in-table table=side-anti-loop 
  ```
  ![SideRouter Policy Routing](/images/side-iprule.png)

至此，你应该已经完成了主路由的策略路由配置：所有来自于旁路由IP的数据包，将仅查询`side-anti-loop`这个路由表，
甚至包括V2Ray配置错误时（例如：错误的将应该代理的流量直连发出）也不会环路，从根本上避免了路由环路的产生。

**⚠️：尽量使用策略路由，在Firewall使用mark-routing（标记路由）很容易会遇到FastTrack不兼容问题，需要额外配置mark-connection/packet较为麻烦**

据我了解，包括FakeDNS以及DNSMasquerade+GFW IPset在内的一众旁路由方案，应该都没有考虑这个问题，对于TCP/UDP以外的流量，会直接产生路由环路。

### 配置DNS转发旁路由

这一步的作用是，主路由拦截所有内网设备发出的DNS请求，并将其转发给旁路由，由旁路由解析并返回，同时做DNS Route决策。
对于整个项目的目标来说，是至关重要的一步，其主要目的是：

* 嗅探内网设备要访问的域名，提前建立路由表转发规则，达成按需转发流量的目的
* 内网设备零配置，对于科学上网完全无感知
* 科学或者旁路故障切换时，仅网关进行切换即可

所以这里的配置就很简单了，只需要排除来源为旁路由IP的DNS查询流量，
然后将所有目的为UDP53的流量DNAT给旁路由即可。

直接上命令，添加DNAT rule，注意替换目标IP为你旁路由的IP，以及，**注意一定要给这条规则，添加注释为：DnsForward**，
这条注释会用作下面探活切换时，防火墙的DNAT规则匹配。

```shell
/ip firewall nat add chain=dstnat protocol=udp dst-port=53 src-address=!192.168.87.2 action=dst-nat to-addresses=192.168.87.2 to-ports=53 comment="DnsForward"
```

**⚠️ 注意，如果你的ROS具有公网地址，则该DNAT配置会导致WAN口UDP53公网可访问，且会响应DNS查询请求。**

这会导致V2Ray的DnsRoute里出现比较奇怪的来源IP记录。要禁止接受WAN口DNS请求，
需要在Firewall - Filter - Forward chain，添加 DST UDP53 且in-interface-list WAN action DROP的规则即可。不再赘述。

![Firewall NAT Rule for DnsForward](/images/firewall-dnsForward.png)

### 配置探活和探活失败时自动回切DNS的脚本

这一步是配置旁路由故障时的自动容灾措施，目的是在旁路由故障时，自动切换DNS为ISP默认DNS，保持主干网络完全可用。

还记得之前在V2Ray的inbounds里，建立了一个protocol名为`http-healthcheck`的代理入口么，那就是本项目用来探测V2Ray实例是否正常工作的探活端点。

相比于IP探活，HTTP探活直接检测了代理软件的存活情况，更加精准可靠。

**以下内容仅适用于ROSv6的系统，v7的系统可以直接使用`Tools -> Netwatch`，直接配置旁路由IP+探活端口，HTTP方式探活即可。**

在ROS的 `System -> Scripts` 菜单中，创建一个名为 `probeSide` 的脚本，内容填写下面的代码。
注意端口号要和V2Ray配置中的探活端口号一致。

```shell
do {
  :local result [/tool fetch url=("http://health-check.side.local:54321/health") mode=http duration=10s output=user as-value];
  :if ($result->"status" = "finished") do={
    :if ([/ip firewall nat get [/ip firewall nat find where comment="DnsForward"] disabled]) do={
      /log info "Side-Router health probe OK - Turn ON DNS Forward";
      /ip firewall nat enable [/ip firewall nat find where comment="DnsForward"];
      /ip dns set allow-remote-requests=no;
    }
  }
} on-error={
  :if (![/ip firewall nat get [/ip firewall nat find where comment="DnsForward"] disabled]) do={
    /log info "Side-Router health probe FAILED - Turn OFF DNS Forward";
    /ip firewall nat disable [/ip firewall nat find where comment="DnsForward"];
    /ip dns set allow-remote-requests=yes;
  }
}
```

然后，在 `IP -> DNS -> Static` 菜单中填入一个静态DNS记录，指向旁路由IP。如下图所示。注意域名不要填错，以及旁路由IP填你自己的IP。

```
health-check.side.local  192.168.87.2
```

![side router static DNS](/images/side-staticdns.png)

最后，在 `System -> Scheduler` 中创建一个定时任务，设置间隔为一分钟，执行下面的命令即可

```shell
/execute script="probeSide"
```

配置完成后如下
![scheduler for side health check](/images/side-healthcheck.png)

至此，你已经完成了主路由和旁路由的拓扑配置，下一步，是时候完成V2Ray的完整配置了。

# V2Ray配置示例

理论上，本方案中V2Ray可按喜好配置GFW黑名单，或者大陆白名单代理模式。
但由于其运行在网关上，如果你不想梯子账单爆炸，一般还是建议使用GFW黑名单+自定义黑名单模式。

**⚠️ 目前本项目只支持V2Ray JSON v4的配置格式，其他格式暂不支持**

### 更新记录

* 2024/08/02: 添加了负载均衡的配置示例，用于简化使用负载均衡作为出口时，conn-track规则书写繁琐的问题。提高配置可维护性。
* 2024/08/15: 添加了statsServer配置，可配合个人修改版的[v2ray-exporter](https://github.com/povsister/v2ray-exporter)/prometheus/grafana观测V2Ray出口及客户端实时流量或趋势。

### V2Ray监控预览
![V2Ray Dashboard](/images/v2ray-dashboard.png)

![V2Ray Dashboard p2](/images/v2ray-dashboard-p2.png)

### GFW黑名单+自定义黑名单配置示例

以下是一个完整的V2Ray配置示例，包含了大陆域名白名单的DNS分流+尝试使用大陆DNS解析未知域名+海外DNS兜底+GFW黑名单路由模式+特殊域名走不同代理出口。
请根据自己需求酌情修改。

```json5
{
  "log": {
    "loglevel": "warning"
  },
  // 最重要的部分
  // 用于DNS route的配置
  "dnsCircuit": {
    //（必填）DNS outbound 的Tag，用于分析DNS请求并预决策路由。
    "dnsOutboundTag": "dns-out",
    //（必填）用于conn-track的inbound，目前只支持dokodemo-door协议的inbound。
    // 填写透明代理的inboundTag即可
    "inboundTags": [
      "transparent"
    ],
    //（outboundTags和balancerTags不能同时为空）
    // 用于conn-track的outbound，同时，DNS路由决策结果命中这些outboundTag的流量都会被转发至旁路由。
    // 填写代理服务器的outboundTag即可
    "outboundTags": [
      "proxy-default",
      "proxy-jp"
    ],
    //（outboundTags和balancerTags不能同时为空）
    // 用于conn-track的balancer，同时，DNS路由决策结果命中这些balancerTag的流量都会被转发至旁路由。
    // 填写出口的balancerTag即可
    "balancerTags": [
      "usa-balancer"
    ],
    //（可选）固定通告某些IP段，目标IP在此范围内的流量都会被转发至旁路由。
    "persistentRoute": [
      // 从规则文件中载入电报的服务器IP段，从而实现内网设备自动通过旁路由访问电报。
      "geoip:telegram"
    ],
    //（可选）不活跃路由的清理时间（秒），不活跃时间超过这个数值后，对应路由条目和conn-track规则会被删除。
    // 默认：21600秒（6个小时）
    "inactiveClean": 21600,
    //（必填）OSPF设置
    // 需要填写软路由和主路由相连的网卡名称，以及软路由自己的IP和网段的掩码，以CIDR形式填写。
    "ospfSetting": {
      //（必填）软路由上的网卡名称，填写你自己的旁路由网卡名称，比如我自己的是ens160
      "ifName": "{IFNAME}",
      //（必填）软路由自己的IP，以及对应的子网掩码
      "address": "192.168.87.2/24"
    }
  },
  "inbounds": [
    {
      // 流量监测用，如果你不需要，删了它
      "tag": "api",
      "listen": "127.0.0.1",
      "port": 11451,
      "protocol": "dokodemo-door",
      "settings": {
        "address": "127.0.0.1"
      }
    },
    {
      //（必填）用作代理软件健康检查
      "tag": "health-check",
      // 必须填写 0.0.0.0 否则无法接受来自旁路由的请求
      "listen": "0.0.0.0",
      // 端口可随意填写，注意和后面主路由配置的健康检查端口对应即可
      "port": 54321,
      //（必填）注意protocol一定要填写http-healthcheck
      "protocol": "http-healthcheck",
      "settings": {
        "timeout": 3
      }
    },
    {
      //（必填）透明代理 inbound，本方案中必须填写
      "tag": "transparent",
      "listen": "127.0.0.1",
      "port": 12345,
      "protocol": "dokodemo-door",
      "settings": {
        "network": "tcp,udp",
        "followRedirect": true
      },
      "sniffing": {
        // 本方案无需开启流量嗅探
        "enabled": false,
      },
      "streamSettings": {
        "sockopt": {
          // 透明代理必须使用 TPROXY 方式
          "tproxy": "tproxy",
          "mark": 255
        }
      }
    }
    // 如果你有主动连接代理软件的需求，
    // 这里还可以继续添加socks或者其他类型的inbounds
  ],
  "outbounds": [
    {
      // 直连流量，也是默认路由出口，
      // 路由策略使用GFW黑名单模式时，请务必将直连作为第一个outbound
      "tag": "direct",
      "protocol": "freedom",
      "settings": {
        "domainStrategy": "UseIPv4"
      },
      "streamSettings": {
        "sockopt": {
          // sock mark不能删，必须和透明代理配置相对应
          "mark": 255
        }
      }
    },
    {
      // 默认代理出口，这里是vmess示例，需要填写你自己的代理服务和对应协议
      "tag": "proxy-default",
      "protocol": "vmess",
      "settings": {
        "vnext": [
          {
            "address": "your.proxy.server",
            "port": 65535,
            "users": [
              {
                "id": "************************",
                "security": "auto"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "sockopt": {
          // sock mark不能删，必须和透明代理配置相对应
          "mark": 255
        }
      }
    },
    {
      //（必填）dns outbound，用于接受DNS请求
      "tag": "dns-out",
      "protocol": "dns",
      "streamSettings": {
        "sockopt": {
          // sock mark不能删，必须和透明代理配置相对应
          "mark": 255
        }
      }
    },
    {
      // 另一个代理出口，用于部分域名按需分流出口。
      // 如果你只有一个代理服务器，可以删掉这里用默认就行。
      // 这里是vmess示例，需要填写你自己的代理服务和对应协议
      "tag": "proxy-jp",
      "protocol": "vmess",
      "settings": {
        "vnext": [
          {
            "address": "your.proxy.server.to.jp",
            "port": 65535,
            "users": [
              {
                "id": "************************",
                "security": "auto"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "sockopt": {
          // sock mark不能删，必须和透明代理配置相对应
          "mark": 255
        }
      }
    },
    // 如果你有多个代理出口分流/负载均衡的需求，
    // 可以在这里继续添加outbounds
    {
      // USA代理出口-1，用于部分域名按需分流出口。
      // 这里是vmess示例，需要填写你自己的代理服务和对应协议
      "tag": "proxy-usa-01",
      "protocol": "vmess",
      "settings": {
        "vnext": [
          {
            "address": "your.proxy.server.to.usa.01",
            "port": 65535,
            "users": [
              {
                "id": "************************",
                "security": "auto"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "sockopt": {
          // sock mark不能删，必须和透明代理配置相对应
          "mark": 255
        }
      }
    },
    {
      // USA代理出口-2，用于部分域名按需分流出口。
      // 这里是vmess示例，需要填写你自己的代理服务和对应协议
      "tag": "proxy-usa-02",
      "protocol": "vmess",
      "settings": {
        "vnext": [
          {
            "address": "your.proxy.server.to.usa.02",
            "port": 65535,
            "users": [
              {
                "id": "************************",
                "security": "auto"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "sockopt": {
          // sock mark不能删，必须和透明代理配置相对应
          "mark": 255
        }
      }
    }
  ],
  // DNS规则，使用了国内外DNS分流+大陆DNS优先+海外DNS兜底的配置
  "dns": {
    "queryStrategy": "UseIPv4",
    // DNS分流必须使用该策略
    "fallbackStrategy": "disabled-if-any-match",
    "domainMatcher": "mph",
    "hosts": {
      // 屏蔽广告域名
      "geosite:category-ads-all": "127.0.0.1",
    },
    // 注意以下配置中，aaa.bbb.ccc.ddd 应该替换为你ISP分配的DNS
    // 如果你使用ROS PPPoE拨号，那么在 IP -> DNS 菜单中，DynamicServers就是你的运营商DNS。
    // 如果你ROS用DHCP上网，那么运营商DNS直接填写光猫网关IP即可。
    "servers": [
      {
        // 默认国内DNS服务器，应该使用你ISP提供的DNS服务器。
        // 用于解析没有命中任何规则的域名，只接受解析结果为大陆IP。
        // 警告：对于未记录大陆域名白名单中的海外域名，这个配置会产生DNS泄露，请自己评估是否可以接受
        "address": "aaa.bbb.ccc.ddd",
        "port": 53,
        "expectIPs": [
          "geoip:cn"
        ],
        "tag": "dns-china-try-resolve",
      },
      {
        // 默认国内DNS服务器的backup，这里使用了DNSPod，你可以按自己喜好调整。
        // 用于解析没有命中任何规则的域名，只接受解析结果为大陆IP。
        // 警告：对于未记录在大陆域名白名单中的海外域名，这个配置会产生DNS泄露，请自己评估是否可以接受
        "address": "119.29.29.29",
        "port": 53,
        "expectIPs": [
          "geoip:cn"
        ],
        "tag": "dns-china-try-resolve-backup",
      },
      {
        // 默认DNS服务器，所有没有命中规则的域名，都会使用这个DNS服务器解析。
        // 示例使用Cloudflare DNS，你可以根据自己喜好调整
        "address": "1.1.1.1",
        "port": 53,
        "tag": "dns-default-abroad"
      },
      {
        // 特殊的域名规则，例如：此配置表示，pixiv的域名，以及jp结尾的域名使用Cloudflare的DNS
        // 然后标记其DNS流量为dns-jp-site，稍后在routing模块中匹配其走JP出口即可，
        "address": "1.1.1.1",
        "port": 53,
        "domains": [
          "geosite:pixiv",
          "regexp:.*\\.jp$"
        ],
        "tag": "dns-jp-site"
      },
      {
        // Twitter / Netflix 等特殊网站DNS规则。
        // 用于分流DNS流量，后续routing模块走特殊出口
        "address": "1.1.1.1",
        "port": 53,
        "domains": [
          "geosite:twitter",
          "geosite:facebook",
          "geosite:netflix"
        ],
        "tag": "dns-usa-site"
      },
      {
        // 特殊域名规则，例如：
        // 群晖的两个顶级域名走114的DNS
        "address": "114.114.114.114",
        "port": 53,
        "domains": [
          "domain:synology.com",
          "domain:synology.cn",
        ],
        "tag": "dns-china-special"
      },
      {
        // 大陆域名白名单，规则中的域名优先使用ISP DNS解析
        "address": "aaa.bbb.ccc.ddd",
        "port": 53,
        "domains": [
          "domain:ntp.org",
          // 用于国内SteamCDN下载
          "domain:steamserver.net",
          "geosite:mihoyo-cn",
          "geosite:china-list",
          "geosite:cn",
          "geosite:tld-cn",
          "geosite:apple",
          "geosite:apple-cn",
          "geosite:google-cn",
          "geosite:icloud",
          "geosite:category-games@cn",
          "geosite:geolocation-cn"
        ],
        "tag": "dns-china-site"
      },
      {
        // 大陆域名白名单的backup，规则中的域名优先使用DNSPod解析
        // 你可以根据自己喜好修改
        "address": "119.29.29.29",
        "port": 53,
        "domains": [
          "domain:ntp.org",
          // 用于国内SteamCDN下载
          "domain:steamserver.net",
          "geosite:mihoyo-cn",
          "geosite:china-list",
          "geosite:cn",
          "geosite:tld-cn",
          "geosite:apple",
          "geosite:apple-cn",
          "geosite:google-cn",
          "geosite:icloud",
          "geosite:category-games@cn",
          "geosite:geolocation-cn"
        ],
        "tag": "dns-china-site-backup"
      },
      {
        // 特殊的域名规则，例如：此配置表示Google的域名使用谷歌的DNS。
        // 然后标记其DNS流量为dns-jp-special，稍后在routing模块中匹配其走JP出口即可。
        // 注意顺序，google-cn需要在这条规则之前，否则google-cn无法直连
        "address": "8.8.8.8",
        "port": 53,
        "domains": [
          "geosite:google"
        ],
        "tag": "dns-jp-special"
      }
    ]
  },
  // 定义多个连接观测器，用于给负载均衡器提供连接观测数据
  "multiObservatory": {
    "observers": [
      {
        // 用default或者填空，另一个burst看了下代码好像没写完
        "type": "default",
        // 定义此连接观测器的tag
        "tag": "internet-usa-observatory",
        "settings": {
          "subjectSelector": [
            // 观测所有proxy-usa开头的outbound
            "proxy-usa"
          ],
          // 采用赛博大善人cloudflare的status api来测试outbound的网络连接情况
          "probeURL": "https://www.cloudflarestatus.com/api/v2/status.json",
          // 每隔多少秒进行一次网络情况测试，不建议设太低，容易浪费流量
          "probeInterval": "60s"
        }
      }
      // 可以继续添加匹配不同出口的连接观测器，用于给多个负载均衡器提供连接观测数据
    ]
  },
  // api配置，用于统计流量
  // 如果你不需要，删了它
  "api": {
    "tag": "api",
    "services": [
      "StatsService"
    ]
  },
  // 别问我为啥要配个空配置，代码里这么要求的。
  // 如果你不需要，删了它
  "stats": {
    // I don't know why this empty config is needed.
    // but without it, the stat server doesn't even boot up.
  },
  // policy开启system outboundlink的上下行统计
  // 如果你不需要，删了它
  "policy": {
    "system": {
      "statsOutboundUplink": true,
      "statsOutboundDownlink": true
    }
  },
  // 此处路由示例路由配置为GFW黑名单+自定义黑名单模式，
  // 不匹配黑名单的域名会默认直连。
  "routing": {
    // 建议使用此域名规则
    "domainStrategy": "IPIfNonMatch",
    "domainMatcher": "mph",
    "balancers": [
      {
        // USA 出口负载均衡，匹配proxy-usa开头的outbound
        "tag": "usa-balancer",
        "selector": [
          "proxy-usa"
        ],
        "strategy": {
          // 可用类型 建议在leastping和random类型中选择，leastload看了下实现好像没做好
          "type": "leastping",
          "settings": {
            // 这里单独定义此负载均衡器，使用上面定义的独立连接观测器。
            // 如果你有多个不同区域的负载均衡，建议为每个负载均衡都使用独立的连接观测器。
            "observerTag": "internet-usa-observatory",
            // 仅random类型的负载均衡有效，leastping会自动检测节点存活情况
            "aliveOnly": true
          }
        },
        // 当负载均衡所有节点均不可用时，降级为直连
        "fallbackTag": "direct"
      }
      // 可以继续添加更多的负载均衡出口，但请记得为每个负载均衡器配置好合适连接观测器和strategy
    ],
    "rules": [
      {
        // 流量观测用，如果你不需要，删了它
        "type": "field",
        "inboundTag": [
          "api"
        ],
        "outboundTag": "api"
      },
      {
        // 直连 123 端口 UDP 流量（NTP 协议）
        "type": "field",
        "inboundTag": "transparent",
        "port": 123,
        "network": "udp",
        "outboundTag": "direct"
      },
      {
        // 劫持 53 端口 UDP 流量，使用 V2Ray 的 DNS
        "type": "field",
        "inboundTag": "transparent",
        "port": 53,
        "network": "udp",
        "outboundTag": "dns-out"
      },
      {
        // 直连 本地保留 ip
        "type": "field",
        "ip": [
          "geoip:private"
        ],
        "outboundTag": "direct"
      },
      {
        // 国内DNS的请求流量直连
        "type": "field",
        "inboundTag": [
          "dns-china-try-resolve",
          "dns-china-try-resolve-backup",
          "dns-china-special",
          "dns-china-site",
          "dns-china-site-backup"
        ],
        "outboundTag": "direct"
      },
      {
        // 特殊规则 使用日本DNS的流量，走日本代理出口
        "type": "field",
        "inboundTag": [
          "dns-jp-special",
          "dns-jp-site"
        ],
        "outboundTag": "proxy-jp"
      },
      {
        // 特殊规则，USA网站的DNS流量，走USA负载均衡出口
        "type": "field",
        "inboundTag": [
          "dns-usa-site"
        ],
        "balancerTag": "usa-balancer"
      },
      {
        // 海外默认DNS产生的流量 走默认代理出口
        "type": "field",
        "inboundTag": [
          "dns-default-abroad"
        ],
        "outboundTag": "proxy-default"
      },
      {
        // 直连 国内网站 保持和上面DNS分流一致即可，
        // 当然，在GFW黑名单路由模式下，这块也可以删掉。填写这个只是为了减少V2Ray的WARNING日志数量。
        "type": "field",
        "domain": [
          "domain:ntp.org",
          // 用于steam下载走国内CDN 如果要steam下载不走代理请务必保留这个规则
          "domain:steamserver.net",
          "geosite:mihoyo-cn",
          "geosite:china-list",
          "geosite:cn",
          "geosite:tld-cn",
          "geosite:apple",
          "geosite:apple-cn",
          "geosite:google-cn",
          "geosite:icloud",
          "geosite:category-games@cn",
          "geosite:geolocation-cn"
        ],
        "outboundTag": "direct"
      },
      {
        // 直连 国内IP
        // 当然，在GFW黑名单路由模式下，这块也可以删掉。
        "type": "field",
        "ip": [
          "geoip:cn"
        ],
        "outboundTag": "direct"
      },
      {
        // Telegram的IP 走默认代理出口
        // 用于和dnsCircuit 的 persistentRoute 相配合，达成内网设备可以直接访问电报
        "type": "field",
        "ip": [
          "geoip:telegram"
        ],
        "outboundTag": "proxy-default"
      },
      {
        // 特殊规则 配合DNS分流，Google的域名+pixiv+jp结尾的域名走日本出口
        "type": "field",
        "domain": [
          "geosite:google",
          "geosite:pixiv",
          "regexp:.*\\.jp$"
        ],
        "outboundTag": "proxy-jp"
      },
      {
        //（重要，必填）
        // 注意顺序，建议紧跟在域名路由规则之后。
        // DNS Route 动态维护的 conn-track 规则，实际使用的是V2Ray router的 srcIP - dstIP 匹配规则。
        // 格式为: 
        // from: dynamic-ipset:dnscircuit-conntrack-src-{outboundTag}
        // to: dynamic-ipset:dnscircuit-conntrack-dest-{outboundTag}
        "type": "field",
        "source": "dynamic-ipset:dnscircuit-conntrack-src-proxy-jp",
        "ip": "dynamic-ipset:dnscircuit-conntrack-dest-proxy-jp",
        "outboundTag": "proxy-jp"
      },
      {
        // USA Twitter Netflix等网站，走USA负载均衡
        "type": "field",
        "domain": [
          "geosite:twitter",
          "geosite:facebook",
          "geosite:netflix"
        ],
        // 注意这里使用了负载均衡作为出口
        "balancerTag": "usa-balancer",
      },
      {
        //（重要，必填）
        // 注意顺序，建议紧跟在域名路由规则之后。
        // DNS Route 动态维护的 conn-track 规则，实际使用的是V2Ray router的 srcIP - dstIP 匹配规则。
        // 格式为: 
        // from: dynamic-ipset:dnscircuit-conntrack-src-{balancerTag}
        // to: dynamic-ipset:dnscircuit-conntrack-dest-{balancerTag}
        "type": "field",
        "source": "dynamic-ipset:dnscircuit-conntrack-src-usa-balancer",
        "ip": "dynamic-ipset:dnscircuit-conntrack-dest-usa-balancer",
        // 注意这里使用了负载均衡作为出口
        "balancerTag": "usa-balancer"
      },
      {
        // 被墙的域名和典型的非大陆域名，走默认代理
        "type": "field",
        "domain": [
          "geosite:gfw",
          "geosite:geolocation-!cn"
        ],
        "outboundTag": "proxy-default"
      },
      {
        //（重要，必填）
        // 注意顺序，建议紧跟在域名路由规则之后。
        // DNS Route 动态维护的 conn-track 规则，实际使用的是V2Ray router的 srcIP - dstIP 匹配规则。
        // 格式为: 
        // from: dynamic-ipset:dnscircuit-conntrack-src-{outboundTag}
        // to: dynamic-ipset:dnscircuit-conntrack-dest-{outboundTag}
        "type": "field",
        "source": "dynamic-ipset:dnscircuit-conntrack-src-proxy-default",
        "ip": "dynamic-ipset:dnscircuit-conntrack-dest-proxy-default",
        "outboundTag": "proxy-default"
      },
      {
        //（重要，必填）
        // 注意顺序，建议写在所有路由规则最后。
        // DNS Route 路由默认出口，当一个incoming连接没有被任何conn-track规则命中时，会被此规则兜底。
        // 照着写即可
        "type": "field",
        "ip": "dynamic-ipset:dnscircuit-dest-default",
        "outboundTag": "proxy-default"
      }
      // 对于剩下未匹配任何路由规则的流量，走默认路由（即第一个outbound）
      // 在GFW黑名单模式下，意即为直连
    ]
  }
}
```

# FAQs

## OSPF 收敛速度快吗？

很快，基本在DNS请求发出后的1秒内就可以完成路由收敛，体感首次访问某个被墙站点时，有30-40%的概率会出现ConnectionRST，
随后只需要刷新一下页面即可正常访问。

同时，由于默认路由有效期是6个小时，对于常用网站，只要不是6个小时内一次没访问过，对应路由规则就会一直生效。

## 这个修改版的V2Ray为什么关闭有点慢

因为使用了OSPF协议，其标准要求，在路由下线时，必须从广播域中废止自己生成的路由条目。
所以，在收到退出信号时，旁路由广播废止路由表后，其实在等待主路由对于废止条目的确认，这个一般需要1-2秒。



