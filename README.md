### This is a modified V2Ray-core maintained by myself.

Updates from the upstream will be merged periodically.

#### It has several added features:
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

## Related Links

- [Documentation](https://www.v2fly.org) and [Newcomer's Instructions](https://www.v2fly.org/guide/start.html)
- Welcome to translate V2Ray documents via [Transifex](https://www.transifex.com/v2fly/public/)

## 为什么开发本项目

先叠个甲，本项目使用繁琐，仅适用于有进阶网络知识的用户使用。

本项目旨在解决：网关透明代理模式下，网关的性能和网络拓扑稳定性的问题。
因此，本项目中的V2Ray将作为旁路由透明代理使用，需要你事先掌握以下内容：
* 什么是透明代理
* 如何配置V2Ray以透明代理模式工作
* 理解单臂路由（旁路由）的基本工作原理
* 具备基本的linux操作能力

核心理念为：仅需要V2Ray处理的流量会被转发至旁路由处理，其余流量由主路由直接发出。

类似按需转发流量的已有实现有：[FakeDNS](https://www.v2fly.org/config/fakedns.html)。
但其存在两个问题：
* FakeIP污染
* 旁路由故障时，对于网络拓扑的影响无法立即消除

相比之下，本方案具有以下优点：
* 全真IP，不存在FakeIP污染
* 旁路完全可插拔，网络拓扑自动切换
* 扩展能力好，可配合V2Ray已有的各种代理协议实现L3组网
* 路由黑白名单灵活配置

## 使用说明

TODO
