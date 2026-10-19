BIN     := podman-tray
APP     := dist/Podman Tray.app
INSTALL := $(HOME)/Applications
ICNS    := packaging/AppIcon.icns
VERSION := $(shell /usr/libexec/PlistBuddy -c "Print CFBundleShortVersionString" packaging/Info.plist)
BUILD   := $(shell date +%Y%m%d%H%M)
DMG     := dist/Podman-Tray-$(VERSION).dmg

# Version (from Info.plist) and build timestamp are embedded in the binary and shown in the menu.
# systray and internal/macos both link -lobjc; Apple's ld warns about the duplicate.
LDFLAGS := -s -w -extldflags=-Wl,-no_warn_duplicate_libraries \
	-X podman-tray/internal/tray.version=$(VERSION) -X podman-tray/internal/tray.build=$(BUILD)

# Signing: ad-hoc by default (fine for this Mac). For distribution, pass a Developer ID
# identity and a notarytool keychain profile:
#   make dmg SIGN_IDENTITY="Developer ID Application: Nome (TEAMID)" NOTARY_PROFILE=podman-tray
SIGN_IDENTITY  ?= -
NOTARY_PROFILE ?=

ifeq ($(SIGN_IDENTITY),-)
CODESIGN := codesign --force --sign -
else
CODESIGN := codesign --force --sign "$(SIGN_IDENTITY)" --options runtime --timestamp
endif

# LaunchServices keeps a record of every copy of the app it has seen. Stale ones (e.g. the
# deleted DMG staging folder) make macOS fail to find the app for notification permissions.
LSREGISTER := /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister

.PHONY: run build universal test vet icon app install dmg uninstall clean

run:
	go run -ldflags="$(LDFLAGS)" ./cmd/$(BIN)

build:
	go build -ldflags="$(LDFLAGS)" -o bin/$(BIN) ./cmd/$(BIN)

# Apple Silicon + Intel in a single binary.
universal:
	CGO_ENABLED=1 GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o bin/$(BIN)-arm64 ./cmd/$(BIN)
	CGO_ENABLED=1 GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/$(BIN)-amd64 ./cmd/$(BIN)
	lipo -create -output bin/$(BIN)-universal bin/$(BIN)-arm64 bin/$(BIN)-amd64

test:
	go test -ldflags="$(LDFLAGS)" ./...

vet:
	go vet ./...

# The .icns is committed; regenerate it after changing tools/genicon.
icon:
	rm -rf bin/AppIcon.iconset
	go run ./tools/genicon bin/AppIcon.iconset
	iconutil -c icns bin/AppIcon.iconset -o $(ICNS)
	cp bin/AppIcon.iconset/icon_128x128@2x.png packaging/AppIcon.png

app: universal
	rm -rf "$(APP)"
	mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources"
	cp bin/$(BIN)-universal "$(APP)/Contents/MacOS/$(BIN)"
	cp packaging/Info.plist "$(APP)/Contents/Info.plist"
	/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $(BUILD)" "$(APP)/Contents/Info.plist"
	cp $(ICNS) "$(APP)/Contents/Resources/AppIcon.icns"
	$(CODESIGN) --entitlements packaging/entitlements.plist "$(APP)"

install: app
	mkdir -p "$(INSTALL)"
	rm -rf "$(INSTALL)/Podman Tray.app"
	cp -R "$(APP)" "$(INSTALL)/"
	touch "$(INSTALL)/Podman Tray.app" # refresh Finder's icon cache
	-$(LSREGISTER) -u "$(CURDIR)/$(APP)"
	$(LSREGISTER) -f "$(INSTALL)/Podman Tray.app"
	open "$(INSTALL)/Podman Tray.app"

# Drag-to-install disk image: the app plus a shortcut to /Applications.
dmg: app
	rm -rf dist/dmg "$(DMG)"
	mkdir -p dist/dmg
	cp -R "$(APP)" dist/dmg/
	ln -s /Applications dist/dmg/Applications
	hdiutil create -volname "Podman Tray" -srcfolder dist/dmg -ov -format UDZO "$(DMG)"
	-$(LSREGISTER) -u "$(CURDIR)/dist/dmg/Podman Tray.app"
	rm -rf dist/dmg
ifneq ($(SIGN_IDENTITY),-)
	codesign --force --sign "$(SIGN_IDENTITY)" --timestamp "$(DMG)"
endif
ifneq ($(NOTARY_PROFILE),)
	xcrun notarytool submit "$(DMG)" --keychain-profile "$(NOTARY_PROFILE)" --wait
	xcrun stapler staple "$(DMG)"
endif
	@echo "→ $(DMG)"

uninstall:
	-pkill -x $(BIN)
	rm -rf "$(INSTALL)/Podman Tray.app"
	rm -f $(HOME)/Library/LaunchAgents/com.podman-tray.plist

clean:
	rm -rf bin dist
