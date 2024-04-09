# DNS-aggregators-golang-fastly-compute

#### 介绍

DNS-aggregators-golang-fastly-compute

#### 软件架构

软件架构说明

#### 使用说明

1. `fastly compute serve`

2. `fastly compute deploy`

# Default Starter Kit for Go

[![Deploy to Fastly](https://deploy.edgecompute.app/button)](https://deploy.edgecompute.app/deploy)

Get to know the Fastly Compute environment with a basic starter that
demonstrates routing, simple synthetic responses and code comments that cover
common patterns.

**For more details about other starter kits for Compute, see the
[Fastly Documentation Hub](https://www.fastly.com/documentation/solutions/starters)**

## Features

- Allow only requests with particular HTTP methods
- Match request URL path and methods for routing
- Build synthetic responses at the edge

## Understanding the code

This starter is intentionally lightweight, and requires no dependencies aside
from the
[`"github.com/fastly/compute-sdk-go/fsthttp"`](https://github.com/fastly/compute-sdk-go)
repo. It will help you understand the basics of processing requests at the edge
using Fastly. This starter includes implementations of common patterns explained
in our [using Compute](https://www.fastly.com/documentation/guides/compute/go/)
and
[VCL migration](https://www.fastly.com/documentation/guides/compute/migrate/)
guides.

The starter doesn't require the use of any backends. Once deployed, you will
have a Fastly service running on Compute that can generate synthetic responses
at the edge.

It is recommended to use the [Fastly CLI](https://github.com/fastly/cli) for
this template. The template uses the `fastly.toml` scripts, to allow for
building the project using your installed Go compiler. The Fastly CLI should
also be used for serving and testing your build output, as well as deploying
your finalized package!

## Security issues

Please see our [SECURITY.md](SECURITY.md) for guidance on reporting
security-related issues.

# 设置上游服务器的地址

在resources的`secret-stores`里面设置名称为"DNS-aggregators-golang-fastly-compute"的`secret stores`里面的"DOH_ENDPOINT"为"https://doh.pub/dns-query",也可以是多个上游服务器的地址的json数组.

# 设置doh服务的路径

在resources的`secret-stores`里面设置名称为"DNS-aggregators-golang-fastly-compute"的`secret stores`里面的"DOH_PATHNAME"为"/dns-query".

访问 "http://127.0.0.1:7676/dns-query" 使用doh服务

# 在控制台的里面添加源站

设置源站的名称为上游服务器的域名

设置网站的Certificate hostname,SNI hostname,Override host为上游服务器的域名

# 设置doh服务的最小ttl(秒)

在resources的`secret-stores`里面设置名称为"DNS-aggregators-golang-fastly-compute"的`secret stores`里面的"DOH_MINTTL"为"600".

为了解决浏览器对于cname记录重复的不兼容问题,需要把cname记录进行展平.
