#!/bin/sh

echo "Copying unit file"
cp /opt/logwatcher/logwatcher.service /etc/systemd/system/logwatcher.service
echo "reloading systemctl"
systemctl daemon-reload
echo "enabling service"
systemctl enable logwatcher.service
systemctl start logwatcher.service
# sleep some time to check if binary crashed
sleep 5
systemctl status logwatcher.service
