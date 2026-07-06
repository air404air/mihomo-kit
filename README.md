# Mihomo Kit for Keenetic + Entware

Состав:
- Mihomo core
- Zashboard на 9090
- SubManager на 9091
- mihomo_redirect для прозрачного TCP-перехвата LAN
- красивый GEOSITE ruleset внутри SubManager

## Как разместить на GitHub

Залей в репозиторий:

```text
install.sh
bin/mihomo-submanager
```

В `install.sh` замени строку:

```sh
BASE_URL="${BASE_URL:-https://raw.githubusercontent.com/air404air/mihomo-kit/main}"
```

на свой raw URL, например:

```sh
BASE_URL="${BASE_URL:-https://raw.githubusercontent.com/air404air/mihomo-kit/main}"
```

## Установка одной командой

```sh
wget -qO- https://raw.githubusercontent.com/air404air/mihomo-kit/main/install.sh | sh
```

или:

```sh
curl -fsSL https://raw.githubusercontent.com/air404air/mihomo-kit/main/install.sh | sh
```

Если не хочешь править `install.sh`, можно указать BASE_URL прямо в команде:

```sh
wget -qO- https://raw.githubusercontent.com/air404air/mihomo-kit/main/install.sh | BASE_URL=https://raw.githubusercontent.com/air404air/mihomo-kit/main sh
```
