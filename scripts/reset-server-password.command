#!/bin/bash
# macOS: double-click to open the server's interactive recovery tool.
echo '正在连接 CNC-Manager 服务器…'
echo '如需密码，请输入服务器 SSH 密码；连接后再输入新的应用密码。'
ssh -t root@10.10.10.23 /opt/cnc-manager-v2/cnc-manager reset-password
cnc_reset_status=$?
echo
read -r -p '按回车关闭窗口…' cnc_reset_reply
exit "$cnc_reset_status"
