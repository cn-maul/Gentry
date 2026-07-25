#!/usr/bin/env python3
"""检查远程容器状态"""
import paramiko, time, sys, os

# 设置 stdout 编码为 UTF-8
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.156', port=22, username='admin', password='Qazay225108', timeout=30)

def run(cmd, timeout=30):
    transport = client.get_transport()
    chan = transport.open_session(timeout=timeout)
    chan.exec_command(f"echo 'Qazay225108' | sudo -S {cmd} 2>&1")
    time.sleep(2)
    out = b""
    while True:
        if chan.exit_status_ready():
            break
        r, w, e = select.select([chan], [], [], 1)
        if r:
            d = chan.recv(8192)
            if d:
                out += d
        else:
            break
    while chan.recv_ready():
        out += chan.recv(8192)
    chan.close()
    return out.decode('utf-8', errors='replace')

import select

time.sleep(8)

# 健康状态
print("=== 健康状态 ===")
print(run("docker inspect gentry --format='{{.State.Health.Status}}'"))

# 容器状态
print("=== 容器状态 ===")
print(run("docker ps --filter name=gentry --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"))

# 日志
print("=== 日志 (最近20行) ===")
print(run("docker logs gentry --tail 20"))

client.close()