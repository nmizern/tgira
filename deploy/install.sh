#!/bin/sh
# Installs tgira on a Debian or Ubuntu host. Run as root, from the directory
# holding the tgira binary.
set -eu

install -d -m 755 /opt/tgira /etc/tgira
install -m 755 tgira /opt/tgira/tgira

if ! id tgira >/dev/null 2>&1; then
    useradd --system --home /var/lib/tgira --create-home --shell /usr/sbin/nologin tgira
fi
install -d -o tgira -g tgira -m 750 /var/lib/tgira

if [ ! -f /etc/tgira/config.yaml ]; then
    install -m 640 config.example.yaml /etc/tgira/config.yaml
    echo "edit /etc/tgira/config.yaml before starting"
fi
if [ ! -f /etc/tgira/tgira.env ]; then
    printf 'TGIRA_TOKEN=\nTGIRA_DB_PATH=/var/lib/tgira/tgira.db\n' > /etc/tgira/tgira.env
    chmod 640 /etc/tgira/tgira.env
    echo "put the BotFather token into /etc/tgira/tgira.env"
fi
chown -R root:tgira /etc/tgira

install -m 644 tgira.service /etc/systemd/system/tgira.service
systemctl daemon-reload

echo "done: systemctl enable --now tgira"
