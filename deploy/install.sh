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

# A config or an env file shipped next to the binary wins over the sample,
# which is what makes an unattended install possible. Neither is ever
# overwritten once it is in place.
if [ ! -f /etc/tgira/config.yaml ]; then
    if [ -f config.yaml ]; then
        install -m 640 config.yaml /etc/tgira/config.yaml
    else
        install -m 640 config.example.yaml /etc/tgira/config.yaml
        echo "edit /etc/tgira/config.yaml before starting"
    fi
fi
if [ ! -f /etc/tgira/tgira.env ]; then
    if [ -f tgira.env ]; then
        install -m 640 tgira.env /etc/tgira/tgira.env
    else
        printf 'TGIRA_TOKEN=\nTGIRA_DB_PATH=/var/lib/tgira/tgira.db\n' > /etc/tgira/tgira.env
        chmod 640 /etc/tgira/tgira.env
        echo "put the BotFather token into /etc/tgira/tgira.env"
    fi
fi
chown -R root:tgira /etc/tgira

install -m 644 tgira.service /etc/systemd/system/tgira.service
systemctl daemon-reload

echo "done: systemctl enable --now tgira"
