BIN     := podman-tray
# Spotlight skips *.noindex folders, so these copies don't show up in Launchpad/Spotlight
# next to the installed app.
APP     := dist/build.noindex/Podman Tray.app
STAGING := dist/dmg.noindex
INSTALL := $(HOME)/Applications
ICNS    := packaging/AppIcon.icns
VERSION := $(shell /usr/libexec/PlistBuddy -c "Print CFBundleShortVersionString" packaging/Info.plist)
BUILD   := $(shell date +%Y%m%d%H%M)
DMG     := dist/Podman-Tray-$(VERSION).dmg

# The version (from Info.plist) is embedded in the binary and shown in the menu; the build
# timestamp only goes into the bundle's CFBundleVersion.
# systray and internal/macos both link -lobjc; Apple's ld warns about the duplicate.
LDFLAGS := -s -w -extldflags=-Wl,-no_warn_duplicate_libraries -X podman-tray/internal/tray.version=$(VERSION)

# Signing, in order of preference:
#  - a Developer ID identity, for distribution (with a notarytool keychain profile):
#      make dmg SIGN_IDENTITY="Developer ID Application: Nome (TEAMID)" NOTARY_PROFILE=podman-tray
#  - the self-signed "Podman Tray Local" identity from `make cert`, picked up automatically:
#    it keeps the app's identity stable across builds on this Mac;
#  - ad-hoc (-) otherwise, e.g. in CI.
LOCAL_IDENTITY := Podman Tray Local
ifeq ($(origin SIGN_IDENTITY),undefined)
SIGN_IDENTITY := $(shell security find-certificate -c "$(LOCAL_IDENTITY)" >/dev/null 2>&1 && echo "$(LOCAL_IDENTITY)" || echo -)
endif
NOTARY_PROFILE ?=

ifeq ($(SIGN_IDENTITY),-)
CODESIGN := codesign --force --sign -
else ifeq ($(SIGN_IDENTITY),$(LOCAL_IDENTITY))
CODESIGN := codesign --force --sign "$(SIGN_IDENTITY)"
else
DEVELOPER_ID := 1
CODESIGN := codesign --force --sign "$(SIGN_IDENTITY)" --options runtime --timestamp
endif

# LaunchServices keeps a record of every copy of the app it has seen. Stale ones (e.g. the
# deleted DMG staging folder) make macOS fail to find the app for notification permissions.
LSREGISTER := /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister

.PHONY: run build universal test vet icon cert app install dmg uninstall clean

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

# Creates the "Podman Tray Local" signing identity in the login keychain (once).
cert:
	tools/local-cert.sh "$(LOCAL_IDENTITY)"

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
	@$(LSREGISTER) -u "$(CURDIR)/$(APP)" 2>/dev/null || true # usually not registered (.noindex)
	$(LSREGISTER) -f "$(INSTALL)/Podman Tray.app"
	open "$(INSTALL)/Podman Tray.app"

# Drag-to-install disk image: the app plus a shortcut to /Applications.
dmg: app
	rm -rf $(STAGING) "$(DMG)"
	mkdir -p $(STAGING)
	cp -R "$(APP)" $(STAGING)/
	ln -s /Applications $(STAGING)/Applications
	hdiutil create -volname "Podman Tray" -srcfolder $(STAGING) -ov -format UDZO "$(DMG)"
	@$(LSREGISTER) -u "$(CURDIR)/$(STAGING)/Podman Tray.app" 2>/dev/null || true
	rm -rf $(STAGING)
ifdef DEVELOPER_ID
	codesign --force --sign "$(SIGN_IDENTITY)" --timestamp "$(DMG)"
endif
ifneq ($(NOTARY_PROFILE),)
	xcrun notarytool submit "$(DMG)" --keychain-profile "$(NOTARY_PROFILE)" --wait
	xcrun stapler staple "$(DMG)"
endif
	@echo "→ $(DMG)"

uninstall:
	-pkill -x $(BIN)
	-"$(INSTALL)/Podman Tray.app/Contents/MacOS/$(BIN)" --unregister-login
	rm -rf "$(INSTALL)/Podman Tray.app"
	rm -f $(HOME)/Library/LaunchAgents/com.podman-tray.plist

clean:
	rm -rf bin dist
