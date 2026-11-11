#!/bin/sh
# Режим входа выбирается до штатного envsubst nginx.
set -eu
case "${AUTH_MODE:-basic}" in basic|session) ;; *) echo 'AUTH_MODE должен быть basic или session' >&2; exit 1;; esac
case "${AUTH_REAL_IP_HEADER:-X-Forwarded-For}" in X-Forwarded-For|X-Real-IP) ;; *) echo 'Некорректный заголовок real IP' >&2; exit 1;; esac
case "${AUTH_LOGIN_BURST:-20}" in ''|*[!0-9]*|0) echo 'Некорректный burst входа' >&2; exit 1;; esac
mkdir -p /etc/nginx/incidentgpt
: > /etc/nginx/incidentgpt/server.conf
: > /etc/nginx/incidentgpt/api.conf
if [ "${AUTH_MODE:-basic}" = session ]; then
    printf '%s\n' 'auth_request /_session_check;' 'proxy_hide_header WWW-Authenticate;' > /etc/nginx/incidentgpt/api.conf
    printf '%s\n' 'proxy_set_header X-Auth-User "";' > /etc/nginx/incidentgpt/identity.conf
else
    if [ "${AUTH_BASIC_ENABLED:-true}" = true ]; then
        printf '%s\n' 'auth_basic "IncidentGPT";' 'auth_basic_user_file /etc/nginx/secrets/htpasswd;' >> /etc/nginx/incidentgpt/server.conf
    fi
    printf '%s\n' 'proxy_set_header X-Auth-User $remote_user;' > /etc/nginx/incidentgpt/identity.conf
fi
# По умолчанию forwarded IP не доверяем. Доверие ограничивается явными CIDR ingress.
if [ -n "${AUTH_TRUSTED_PROXY_CIDRS:-}" ]; then
    printf 'real_ip_header %s;\nreal_ip_recursive on;\n' "$AUTH_REAL_IP_HEADER" >> /etc/nginx/incidentgpt/server.conf
    for cidr in $AUTH_TRUSTED_PROXY_CIDRS; do
        case "$cidr" in *[!0-9a-fA-F:./]*) echo 'Некорректный CIDR доверенного прокси' >&2; exit 1;; esac
        printf 'set_real_ip_from %s;\n' "$cidr" >> /etc/nginx/incidentgpt/server.conf
    done
fi
