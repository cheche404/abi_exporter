# abi_exporter
```text
调用一个接口，接口返回项目中证书的过期时间。将证书过期时期封闭为 Prometheus 指标。方便后续监控和告警。
```
```shell
# 构建二进制文件
go build -o abi_exporter main.go
```