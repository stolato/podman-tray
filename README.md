# Podman Tray

App de bandeja (barra de menu) para **macOS, Linux e Windows**, escrito em Go, para listar e controlar containers do Podman sem abrir o terminal.

<img src="packaging/AppIcon.png" width="128" alt="Ícone do Podman Tray">

## Recursos

- **Lista de containers** (`podman ps -a`) com status — 🟢 rodando, 🟠 rodando mas *unhealthy* (healthcheck falhando), 🟡 pausado/reiniciando, ⚪️ parado, 🔴 erro, ⏳ ação em andamento — e contador `rodando/total` no topo do menu. O ícone ganha uma **bolinha verde** enquanto algum container roda
- **Submenu por container**:
  - status, **CPU/memória** e imagem
  - **portas clicáveis**: `🌐 localhost:8080 → 80/tcp` abre `http://localhost:8080` no navegador (portas UDP aparecem, mas não são clicáveis)
  - **Start**, **Stop**, **Restart** e **Pause**/**Unpause**
  - **Logs…** (`logs -f`), **Shell…** (`exec -it`, usa `bash` se existir, senão `sh`) e **Inspect…**, abertos num terminal (veja [Diferenças entre os sistemas](#diferenças-entre-os-sistemas))
  - **📋 Copiar**: ID, nome e as URLs das portas
  - **🗑 Remover…** (`rm -f`), com confirmação
- **Grupos**: containers de um mesmo projeto do compose (label `com.docker.compose.project` ou `io.podman.compose.project`) ou de um mesmo pod ficam juntos num item `🟢 projeto (2/3)` (🟢 todos rodando, 🟡 parte, ⚪️ nenhum, 🟠 algum *unhealthy*), com os containers no submenu e **Iniciar grupo** / **Parar grupo**. Pods usam `podman pod start/stop`; projetos do compose agem em cada container. Se o container tem os dois (o `podman-compose` cria um pod `pod_<projeto>`), vale o projeto. Containers *infra* dos pods não aparecem
- **Remover grupo**, com confirmação: **🗑 Remover pod…** roda `podman pod rm -f` (o pod e todos os containers dele). **🗑 Remover containers…** remove os containers de um projeto do compose e o pod `pod_<projeto>` do `podman-compose`; como o app não tem o arquivo do compose, não é um `compose down`: redes ficam, e um `compose up` recria tudo. Nenhuma remoção apaga volumes
- **Iniciar todos** / **Parar todos**: roda em paralelo e manda uma única notificação de resumo (containers pausados são retomados com `unpause` ao iniciar e parados ao parar)
- **Podman machine**: mostra o estado (🟢 rodando, 🟡 iniciando, ⚪️ parada), nome, CPUs e memória; só habilita **Iniciar** com a machine parada e **Parar** com ela rodando, e pede confirmação antes de parar
- **Alertas**: notifica quando um container **cai** (sai com código diferente de 0 sem ter sido parado por alguém — `podman stop` de qualquer lugar não conta; falta de memória aparece como OOM) e quando fica **unhealthy**. No máximo um alerta a cada 5 min por container, para um container em loop de restart não inundar a central de notificações
- **🧹 Limpeza**: espaço usado por imagens e volumes (`podman system df`, atualizado no máximo a cada minuto, enquanto o menu está em uso), **Remover imagens sem uso…** (`podman image prune --all`) e **Remover pods vazios…** (como os que o `podman-compose` deixa para trás), com confirmação. Volumes guardam dados, então não há limpeza deles pelo menu
- **Notificações** nativas do sistema ao concluir ações ou em caso de erro. **⚙️ Opções → Notificar ações concluídas** desliga as de sucesso; erros e alertas continuam. A escolha fica em `settings.json`, na pasta de configuração do usuário: `~/Library/Application Support/com.podman-tray/` (macOS), `~/.config/com.podman-tray/` (Linux) ou `%AppData%\com.podman-tray\` (Windows). Se as notificações do app estiverem desligadas nas configurações do sistema (macOS e Windows), o menu mostra **🔕 Notificações desativadas, abrir configurações**
- **Aviso de nova versão**: consulta o último release no GitHub ao abrir e uma vez por dia, e mostra **⬆️ Nova versão X disponível** no fim do menu (abre a página do release). Builds de desenvolvimento não consultam
- **⚙️ Opções → Abrir ao iniciar sessão**: no macOS 13+ registra o app em **Ajustes do Sistema → Geral → Itens de Início** (`SMAppService`); no macOS 12, ou fora do `.app`, usa um LaunchAgent (`~/Library/LaunchAgents/com.podman-tray.plist`). No Linux usa `~/.config/autostart` e no Windows a chave `Run` do registro

### Como a lista se mantém atualizada

- O app escuta `podman events` e atualiza o menu assim que um container muda de estado. Com o stream ativo, o polling cai para 60s, só como garantia.
- Sem o stream, faz polling a cada 5s. Se o podman estiver inacessível (ex.: machine parada), as tentativas se espaçam de 5s até 60s.
- CPU/memória (`podman stats`) só são buscados enquanto o menu está em uso: a cada 2s durante os 30s após abrir o menu. A CPU é calculada pela variação de `cpu_time` entre amostras, ou seja, é o uso atual.
- No Windows a bandeja não avisa quando o menu abre, então CPU/memória e uso de disco são buscados a cada atualização normal da lista (sem o ritmo de 2s), e a CPU mostrada é a média que o próprio podman calcula.

## Requisitos

- [Podman](https://podman.io) instalado
- **macOS** 12 ou superior, com uma Podman machine (Homebrew, instalador oficial ou `/usr/local/bin`)
- **Linux** com bandeja do sistema (StatusNotifierItem): KDE, XFCE, Cinnamon, Ubuntu e a maioria dos ambientes. No GNOME puro (ex.: Fedora), instale a extensão [AppIndicator and KStatusNotifierItem Support](https://extensions.gnome.org/extension/615/appindicator-support/). Opcionais: `zenity` ou `kdialog` (confirmações), `wl-clipboard`, `xclip` ou `xsel` (copiar)
- **Windows** 10 ou 11, com uma Podman machine (instalador oficial)
- Go 1.27+ (só para compilar)

## Instalação

### macOS

```bash
make install
```

Compila, gera `dist/build.noindex/Podman Tray.app`, copia para `~/Applications` e abre. Para ligar a inicialização automática, marque **⚙️ Opções → Abrir ao iniciar sessão** no menu.

Na primeira execução o macOS pede permissão para o app enviar notificações. Na primeira vez que usar **Logs…**, **Shell…** ou **Inspect…**, pede também permissão para controlar o Terminal.

Para remover (app, login item e LaunchAgent):

```bash
make uninstall
```

### Linux

Baixe `podman-tray-<versão>-linux-<amd64|arm64>.tar.gz` do [release](https://github.com/stolato/podman-tray/releases) (ou gere com `make linux`) e rode:

```bash
tar -xzf podman-tray-*-linux-*.tar.gz && cd podman-tray-*/ && ./install.sh
```

Instala só para o seu usuário, sem root: o binário em `~/.local/bin`, o atalho no menu de aplicativos e o ícone. **⚙️ Opções → Abrir ao iniciar sessão** cria `~/.config/autostart/podman-tray.desktop`. Para remover: `./install.sh --uninstall`.

### Windows

Baixe `podman-tray-<versão>-windows-<amd64|arm64>.zip` do [release](https://github.com/stolato/podman-tray/releases) (ou gere com `make windows`), extraia o `podman-tray.exe` numa pasta fixa (ex.: `%LOCALAPPDATA%\Podman Tray`) e abra. O executável não é assinado, então o SmartScreen avisa na primeira vez: **Mais informações → Executar assim mesmo**. **⚙️ Opções → Abrir ao iniciar sessão** usa a chave `Run` do registro (aparece em **Gerenciador de Tarefas → Aplicativos de inicialização**).

### Diferenças entre os sistemas

| | macOS | Linux | Windows |
|---|---|---|---|
| Notificações | Central de Notificações | D-Bus (`org.freedesktop.Notifications`) | toast do Windows (via PowerShell) |
| Confirmações | alerta nativo | `zenity` ou `kdialog`; sem eles, ações destrutivas são recusadas | caixa de mensagem (OK/Cancelar) |
| Logs/Shell/Inspect | Terminal.app | `$TERMINAL`, `x-terminal-emulator`, GNOME Terminal, Konsole, Kitty, Alacritty… | Windows Terminal, ou `cmd` |
| Abrir ao iniciar sessão | `SMAppService` (Itens de Início) | `~/.config/autostart` | registro `HKCU\…\Run` |
| Podman machine | sim | só aparece se existir uma (o podman roda nativo) | sim |
| CPU/memória | ao abrir o menu (a cada 2s) | ao abrir o menu (a cada 2s) | a cada atualização: o Windows não avisa quando o menu abre |
| Ícone | segue o tema claro/escuro da barra | pod branco com contorno, para fundo claro ou escuro | idem |

## Distribuição no macOS (DMG)

```bash
make dmg   # → dist/Podman-Tray-<versão>.dmg
```

Gera um DMG de arrastar para instalar (o app + atalho para `/Applications`), com binário universal (Apple Silicon + Intel).

**Versão.** A versão vem de `CFBundleShortVersionString` no `packaging/Info.plist`: altere lá antes de gerar um release. Cada build recebe também um número com data e hora (`CFBundleVersion`, ex. `202610152041`). A versão aparece no fim do menu (`Podman Tray 1.3.0`) e no tooltip do ícone, em todos os sistemas, e é ela que o aviso de nova versão compara com o último release; o build fica no **Obter Informações** do Finder.

**Assinatura local.** Rode uma vez:

```bash
make cert
```

Cria a identidade autoassinada **Podman Tray Local** no chaveiro de login (`tools/local-cert.sh`), e os builds passam a usá-la automaticamente. Com assinatura *ad-hoc*, cada build tem uma identidade diferente, e o macOS pode tratar cada `make install` como um app novo (pedir de novo a permissão do Terminal, duplicar o login item); com o certificado local ela é sempre a mesma. Só vale para este Mac. Sem o certificado (por exemplo no CI), o build volta para *ad-hoc*.

**Assinatura para distribuir.** O DMG do CI tem assinatura *ad-hoc*: funciona, mas em outro Mac o Gatekeeper bloqueia o app baixado. Para liberar, quem recebe vai em **Ajustes do Sistema → Privacidade e Segurança → Abrir Mesmo Assim**, ou roda:

```bash
xattr -dr com.apple.quarantine "/Applications/Podman Tray.app"
```

Para distribuir sem esse aviso, é preciso uma conta no [Apple Developer Program](https://developer.apple.com/programs/) (US$ 99/ano):

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

## Desenvolvimento

```bash
make run        # roda direto com go run
make test       # testes
make vet        # go vet
make build      # só o binário nativo em bin/podman-tray
make universal  # binário universal em bin/podman-tray-universal
make app        # gera "dist/build.noindex/Podman Tray.app" (universal; .noindex tira do Spotlight)
make cert       # cria o certificado local de assinatura (uma vez)
make icon       # regenera packaging/AppIcon.icns (e o .png do README) a partir de tools/genicon
make linux      # dist/podman-tray-<versão>-linux-{amd64,arm64}.tar.gz
make windows    # dist/podman-tray-<versão>-windows-{amd64,arm64}.zip
make clean      # remove bin/ e dist/
```

Os builds de Linux e Windows são Go puro (sem cgo), então `make linux windows` funciona em qualquer sistema. O CI (`.github/workflows/build.yml`) gera os três, roda os testes também no Linux e no Windows, e numa tag `v*` publica tudo no release.

A opção **Abrir ao iniciar sessão** fica desabilitada no `make run`, porque o binário do `go run` vive num diretório temporário.

### Estrutura

```
cmd/podman-tray/      # main: inicia o systray (ou --unregister-login, usado pelo make uninstall)
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

## Solução de problemas

- **"podman não encontrado no PATH"**: apps abertos pelo Finder (ou pelo autostart) podem receber um PATH mínimo. O app procura também nos lugares comuns de cada sistema; se o podman estiver em outro lugar, adicione o caminho em `internal/podman/paths_<sistema>.go`.
- **Linux: o ícone não aparece**: o ambiente não tem bandeja StatusNotifierItem. No GNOME, instale a extensão AppIndicator (veja Requisitos).
- **Linux: "Instale o zenity ou o kdialog"**: sem um deles o app não consegue pedir confirmação, então recusa remover ou parar a machine.
- **Windows: notificações não aparecem**: confira **Configurações → Sistema → Notificações → Podman Tray**; o menu mostra **🔕 Notificações desativadas** quando estão desligadas lá.
- **"⚠️ Podman indisponível"**: normalmente a machine está parada. Use **▶ Iniciar Podman machine** no menu.
- **Notificações com o ícone do Editor de Scripts**: o app só envia notificações próprias (com o ícone dele) quando tem permissão; sem ela, usa o `osascript`. Libere em **Ajustes do Sistema → Notificações → Podman Tray** — vale na hora, sem reiniciar. No `make run` (fora do `.app`) é sempre `osascript`.
  Se o pedido de permissão nunca aparece, o macOS pode estar com um registro antigo do app: `make install` já limpa isso, mas dá para conferir com `lsregister -dump | grep -A3 com.podman-tray`.
- **Ícone genérico no Finder/Dock** depois de instalar: cache do macOS. Rode `killall Finder Dock`.
- **Vários "Podman Tray" no Launchpad/Spotlight**: há mais de uma cópia do app. O build já fica fora do Spotlight (`dist/*.noindex`), então sobram instalações antigas, por exemplo em `/Applications` além de `~/Applications`. Para achar todas: `mdfind "kMDItemCFBundleIdentifier == 'com.podman-tray'"`. Apague as que sobram e tire o registro delas com `lsregister -u <caminho>` (o `lsregister` fica em `/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/`).
- **Login item depois de mover o app**: no macOS 13+ o login item segue o app. No macOS 12, o LaunchAgent abre o app pelo bundle id (`open -b com.podman-tray`), então mover o app também não quebra nada. Fora do bundle, o app reescreve o LaunchAgent sozinho ao iniciar.
- **Migração do LaunchAgent**: versões antigas usavam sempre o LaunchAgent. No macOS 13+, ao iniciar, o app registra o login item pelo `SMAppService` e apaga o plist antigo.
- **"Abrir ao iniciar sessão" não fica marcado**: o login item foi desativado em **Ajustes do Sistema → Geral → Itens de Início**. O app abre essa tela; ative o Podman Tray lá.
