#!/usr/bin/env python3
"""
Gentry 部署脚本 — 通过 SSH 上传并部署 Docker 服务到远程服务器
使用 admin 用户登录，sudo 提权执行 Docker 命令
"""

import paramiko
import os
import sys
import time

# ====== 配置 ======
REMOTE_HOST = "192.168.2.156"
REMOTE_PORT = 22
REMOTE_USER = "admin"
REMOTE_PASS = "Qazay225108"
REMOTE_DIR = "/home/admin/gentry"

LOCAL_DEPLOY_DIR = os.path.dirname(os.path.abspath(__file__))

def print_step(msg):
    print(f"\n{'='*60}")
    print(f"  >>> {msg}")
    print(f"{'='*60}")

def connect_ssh():
    """建立 SSH 连接"""
    print_step("连接远程服务器")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        REMOTE_HOST,
        port=REMOTE_PORT,
        username=REMOTE_USER,
        password=REMOTE_PASS,
        timeout=30
    )
    print("  [OK] SSH 连接成功（admin 用户）")
    return client

def sudo_run(client, command, check=True):
    """使用 sudo 执行命令（通过管道输入密码）"""
    full_cmd = f"echo '{REMOTE_PASS}' | sudo -S {command}"
    stdin, stdout, stderr = client.exec_command(full_cmd, timeout=120)
    exit_code = stdout.channel.recv_exit_status()
    out = stdout.read().decode("utf-8", errors="replace").strip()
    err = stderr.read().decode("utf-8", errors="replace").strip()
    if out:
        print(out)
    if err and "password" not in err.lower() and "sorry" not in err.lower():
        print(f"  [stderr] {err}")
    if check and exit_code != 0:
        print(f"  [!] 命令退出码: {exit_code}")
    return exit_code, out, err

def run_command(client, command, check=True):
    """在远程执行普通命令"""
    stdin, stdout, stderr = client.exec_command(command, timeout=120)
    exit_code = stdout.channel.recv_exit_status()
    out = stdout.read().decode("utf-8", errors="replace").strip()
    err = stderr.read().decode("utf-8", errors="replace").strip()
    if out:
        print(out)
    if err:
        print(f"  [stderr] {err}")
    if check and exit_code != 0:
        print(f"  [!] 命令退出码: {exit_code}")
    return exit_code, out, err

def upload_files(client):
    """通过 SFTP 上传部署文件"""
    print_step("上传部署文件到远程服务器")
    sftp = client.open_sftp()

    # 确保远程目录存在
    parts = REMOTE_DIR.split("/")
    path = ""
    for part in parts:
        if not part:
            continue
        path = f"{path}/{part}"
        try:
            sftp.stat(path)
        except FileNotFoundError:
            sftp.mkdir(path)
            print(f"  [OK] 创建目录 {path}")

    # 上传 deploy 目录下的所有文件（排除 deploy.py 自身）
    files_to_upload = [f for f in os.listdir(LOCAL_DEPLOY_DIR)
                       if os.path.isfile(os.path.join(LOCAL_DEPLOY_DIR, f))
                       and f != "deploy.py"]

    for fname in files_to_upload:
        local_path = os.path.join(LOCAL_DEPLOY_DIR, fname)
        remote_path = f"{REMOTE_DIR}/{fname}"
        print(f"  上传 {fname}...")
        sftp.put(local_path, remote_path)
        size_mb = os.path.getsize(local_path) / 1024 / 1024
        print(f"    [OK] {fname} ({size_mb:.1f} MB)")

    sftp.close()
    print("  [OK] 所有文件上传完成")

def check_docker(client):
    """检查远程 Docker 环境"""
    print_step("检查 Docker 环境")
    code, out, _ = sudo_run(client, "docker --version", check=False)
    if code != 0:
        print("  [!] Docker 未安装，尝试安装 Docker...")
        sudo_run(client, "apt-get update -qq && apt-get install -y -qq docker.io docker-compose-v2 2>&1 | tail -5")
        sudo_run(client, "systemctl start docker || true")
        sudo_run(client, "docker --version")
    else:
        print(f"  [OK] {out}")

    code2, out2, _ = sudo_run(client, "docker compose version", check=False)
    if code2 == 0:
        print(f"  [OK] {out2}")
    else:
        print("  [!] docker compose 不可用，尝试安装...")
        sudo_run(client, "apt-get install -y -qq docker-compose-v2 2>&1 | tail -3")

def build_and_deploy(client):
    """在远程构建 Docker 镜像并启动服务"""
    print_step("构建 Docker 镜像")
    sudo_run(client, f"cd {REMOTE_DIR} && docker build -t gentry:latest -f Dockerfile .")

    print_step("停止旧容器（如有）")
    sudo_run(client, "docker stop gentry 2>/dev/null || true", check=False)
    sudo_run(client, "docker rm gentry 2>/dev/null || true", check=False)

    print_step("启动新容器")
    sudo_run(client, f"cd {REMOTE_DIR} && docker compose up -d")

    print_step("等待服务启动")
    time.sleep(5)
    sudo_run(client, "docker ps --filter name=gentry --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'")

    print_step("检查健康状态")
    code, out, _ = sudo_run(client, "docker inspect gentry --format='{{.State.Health.Status}}' 2>/dev/null || echo 'no healthcheck'", check=False)
    print(f"  健康状态: {out}")

def main():
    print("=" * 60)
    print("  Gentry 部署脚本")
    print(f"  目标: {REMOTE_USER}@{REMOTE_HOST}:{REMOTE_PORT}")
    print(f"  目录: {REMOTE_DIR}")
    print("=" * 60)

    client = None
    try:
        client = connect_ssh()
        upload_files(client)
        check_docker(client)
        build_and_deploy(client)

        print_step("部署完成！")
        print(f"  访问 http://{REMOTE_HOST}:8889 进入管理界面")
        print(f"  查看日志: sudo docker logs -f gentry")
        print(f"  停止服务: cd {REMOTE_DIR} && sudo docker compose down")
    except Exception as e:
        print(f"\n!! 部署失败: {e}")
        sys.exit(1)
    finally:
        if client:
            client.close()

if __name__ == "__main__":
    main()