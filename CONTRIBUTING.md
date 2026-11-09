# Contribuindo com o Podman Tray

Issues e pull requests são bem-vindos. Antes de abrir um PR, rode `make vet test`.

## Requisitos

- Go 1.27+
- [Podman](https://podman.io), para testar de verdade
- macOS: Xcode Command Line Tools (o build do macOS usa cgo)

## Comandos

```bash
make run        # roda direto com go run
make test       # testes
make vet        # go vet
make build      # só o binário nativo em bin/podman-tray
make universal  # binário universal em bin/podman-tray-universal
make app        # gera "dist/build.noindex/Podman Tray.app" (universal; .noindex tira do Spotlight)
make install    # gera o .app, copia para ~/Applications e abre (macOS)
make uninstall  # remove o app, o login item e o LaunchAgent (macOS)
make cert       # cria o certificado local de assinatura (uma vez)
make icon       # regenera packaging/AppIcon.icns (e o .png do README) a partir de tools/genicon
make dmg        # dist/Podman-Tray-<versão>.dmg
make linux      # dist/podman-tray-<versão>-linux-{amd64,arm64}.tar.gz
make windows    # dist/podman-tray-<versão>-windows-{amd64,arm64}.zip
make clean      # remove bin/ e dist/
```

Os builds de Linux e Windows são Go puro (sem cgo), então `make linux windows` funciona em qualquer sistema. O CI (`.github/workflows/build.yml`) gera os três, roda os testes também no Linux e no Windows, e numa tag `v*` publica tudo no release.

A opção **Abrir ao iniciar sessão** fica desabilitada no `make run`, porque o binário do `go run` vive num diretório temporário.

## Estrutura

```
cmd/podman-tray/      # main: inicia o systray (ou --unregister-login, usado pelo make uninstall)
internal/i18n/        # traduções: textos em inglês no código, pt.go com o português; idioma do sistema (lang_*.go)
internal/podman/      # wrapper do CLI do podman: containers, stats, events, machine (paths_*.go: onde procurar o podman em cada sistema)
internal/desktop/     # integrações com o sistema, uma implementação por SO (*_darwin, *_linux, *_windows):
                      #   notificações, confirmação, terminal, navegador, área de transferência, abrir ao iniciar sessão
internal/tray/
  app.go              # ciclo de vida e loop de atualização (intervalos e backoff)
  events.go           # stream de `podman events` com reconexão e debounce; alertas de queda e healthcheck
  menu.go             # montagem e atualização do menu
  actions.go          # ações de container (start/stop/restart/pause, remover, copiar), em lote e de grupo, limpeza, machine, opções
  group.go            # agrupamento por projeto do compose / pod
  settings.go         # preferências salvas (notificar ações concluídas)
  update.go           # aviso de nova versão (releases do GitHub)
  stats.go            # cálculo de CPU/memória
  icon.go             # ícone da barra de menu (desenhado em código)
  trayicon_*.go       # como o ícone é aplicado: máscara no macOS, colorido (e .ico no Windows) nos outros
tools/genicon/        # gerador do ícone do app
tools/local-cert.sh   # certificado local de assinatura (make cert)
packaging/            # Info.plist, entitlements.plist e AppIcon.icns do bundle (+ AppIcon.png do README)
packaging/linux/      # .desktop e install.sh do pacote Linux
```

O menu só é recriado quando muda o conjunto de containers (seus grupos ou portas) ou o estado de erro; nas demais atualizações, os itens existentes são editados no lugar.

### Como a lista se mantém atualizada

- O app escuta `podman events` e atualiza o menu assim que um container muda de estado. Com o stream ativo, o polling cai para 60s, só como garantia.
- Sem o stream, faz polling a cada 5s. Se o podman estiver inacessível (ex.: machine parada), as tentativas se espaçam de 5s até 60s.
- CPU/memória (`podman stats`) só são buscados enquanto o menu está em uso: a cada 2s durante os 30s após abrir o menu. A CPU é calculada pela variação de `cpu_time` entre amostras, ou seja, é o uso atual.
- No Windows a bandeja não avisa quando o menu abre, então CPU/memória e uso de disco são buscados a cada atualização normal da lista (sem o ritmo de 2s), e a CPU mostrada é a média que o próprio podman calcula.

## Traduções

Os textos ficam em inglês no código, sempre dentro de `i18n.T("...")` (ou `i18n.F("... %d", n)` quando têm valores), e o português fica em `internal/i18n/pt.go`. O idioma segue o do sistema; para testar outro, rode com `PODMAN_TRAY_LANG=en make run` (ou `pt-BR`).

O `TestCatalog` falha se um texto novo não tiver tradução, se a tradução não tiver os mesmos `%d`/`%s`, ou se sobrar tradução sem uso. Para incluir outro idioma, crie um `xx.go` com o mesmo formato do `pt.go` e adicione o idioma em `pick` (`internal/i18n/i18n.go`).

Os textos fora do Go também têm versão em português: `packaging/*.lproj/InfoPlist.strings` (o pedido de permissão do Terminal no macOS) e o `Comment[pt_BR]` dos `.desktop` do Linux.

## Versão e release

A versão vem de `CFBundleShortVersionString` no `packaging/Info.plist`: altere lá antes de gerar um release. Cada build recebe também um número com data e hora (`CFBundleVersion`, ex. `202610152041`). A versão aparece no fim do menu (`Podman Tray 1.4.0`) e no tooltip do ícone, em todos os sistemas, e é ela que o aviso de nova versão compara com o último release; o build fica no **Obter Informações** do Finder.

Para publicar, crie e envie uma tag `v<versão>`: o CI gera o DMG, os `.tar.gz`, os `.zip` e o `SHA256SUMS`, e cria o release.

O Dependabot (`.github/dependabot.yml`) abre PRs semanais para os módulos Go e mensais para as GitHub Actions, agrupados num PR cada.

## Assinatura no macOS

**Local.** Rode uma vez:

```bash
make cert
```

Cria a identidade autoassinada **Podman Tray Local** no chaveiro de login (`tools/local-cert.sh`), e os builds passam a usá-la automaticamente. Com assinatura *ad-hoc*, cada build tem uma identidade diferente, e o macOS pode tratar cada `make install` como um app novo (pedir de novo a permissão do Terminal, duplicar o login item); com o certificado local ela é sempre a mesma. Só vale para este Mac. Sem o certificado (por exemplo no CI), o build volta para *ad-hoc*.

**Para distribuir.** O DMG do CI tem assinatura *ad-hoc*: funciona, mas em outro Mac o Gatekeeper bloqueia o app baixado (veja a [instalação no README](README.md#macos)). Para distribuir sem esse aviso, é preciso uma conta no [Apple Developer Program](https://developer.apple.com/programs/) (US$ 99/ano):

1. Crie um certificado **Developer ID Application** (Xcode → Settings → Accounts → Manage Certificates).
2. Salve as credenciais do notarytool uma vez (use uma *app-specific password* do appleid.apple.com):
   ```bash
   xcrun notarytool store-credentials podman-tray --apple-id voce@exemplo.com --team-id TEAMID
   ```
3. Gere o DMG assinado, notarizado e com o ticket anexado (*stapled*):
   ```bash
   make dmg SIGN_IDENTITY="Developer ID Application: Seu Nome (TEAMID)" NOTARY_PROFILE=podman-tray
   ```

Com identidade, o app é assinado com *hardened runtime* e o entitlement `com.apple.security.automation.apple-events` (`packaging/entitlements.plist`), necessário para abrir o Terminal via AppleScript.

## Problemas de desenvolvimento no macOS

- **Notificações com o ícone do Editor de Scripts**: no `make run` (fora do `.app`) as notificações sempre usam o `osascript`.
- **O pedido de permissão de notificação nunca aparece**: o macOS pode estar com um registro antigo do app. `make install` já limpa isso, mas dá para conferir com `lsregister -dump | grep -A3 com.podman-tray`.
- **Vários "Podman Tray" no Launchpad/Spotlight**: há mais de uma cópia do app. O build já fica fora do Spotlight (`dist/*.noindex`), então sobram instalações antigas, por exemplo em `/Applications` além de `~/Applications`. Para achar todas: `mdfind "kMDItemCFBundleIdentifier == 'com.podman-tray'"`. Apague as que sobram e tire o registro delas com `lsregister -u <caminho>` (o `lsregister` fica em `/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/`).
- **Se o podman estiver num lugar incomum**: adicione o caminho em `internal/podman/paths_<sistema>.go`.
