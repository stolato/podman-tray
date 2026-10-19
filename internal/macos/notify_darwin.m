#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>
#include <stdbool.h>

#include "_cgo_export.h"

// Shows banners even when the app counts as frontmost (e.g. right after a menu click).
@interface PTNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation PTNotificationDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
  completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
                    UNNotificationPresentationOptionSound);
}
@end

static PTNotificationDelegate *delegate;

// UNUserNotificationCenter throws outside an app bundle (e.g. `go run`).
bool pt_in_bundle(void) {
  return [[NSBundle mainBundle] bundleIdentifier] != nil;
}

// Asks for permission; macOS only prompts the first time.
void pt_notifications_setup(void) {
  UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
  delegate = [PTNotificationDelegate new];
  center.delegate = delegate;
  [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                        completionHandler:^(BOOL granted, NSError *error) {
                          if (error != nil) {
                            NSLog(@"podman-tray: notification authorization: %@", error);
                          }
                        }];
}

// Posts a notification as the app. Permission is checked every time, so enabling it in
// System Settings takes effect without a restart; until then it falls back to osascript.
void pt_notify(const char *title, const char *body) {
  NSString *t = [NSString stringWithUTF8String:title];
  NSString *b = [NSString stringWithUTF8String:body];
  UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
  [center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
    UNAuthorizationStatus status = settings.authorizationStatus;
    if (status != UNAuthorizationStatusAuthorized && status != UNAuthorizationStatusProvisional) {
      ptNotifyFallback((char *)t.UTF8String, (char *)b.UTF8String);
      return;
    }
    UNMutableNotificationContent *content = [UNMutableNotificationContent new];
    content.title = t;
    content.body = b;
    UNNotificationRequest *req = [UNNotificationRequest requestWithIdentifier:[[NSUUID UUID] UUIDString]
                                                                      content:content
                                                                      trigger:nil];
    [center addNotificationRequest:req withCompletionHandler:nil];
  }];
}

bool pt_confirm(const char *title, const char *msg, const char *button) {
  NSString *t = [NSString stringWithUTF8String:title];
  NSString *m = [NSString stringWithUTF8String:msg];
  NSString *b = [NSString stringWithUTF8String:button];
  __block NSModalResponse resp = NSAlertFirstButtonReturn;

  void (^show)(void) = ^{
    // Menu bar apps aren't frontmost; without this the alert opens behind other windows.
    if (@available(macOS 14.0, *)) {
      [NSApp activate];
    } else {
      [NSApp activateIgnoringOtherApps:YES];
    }
    NSAlert *alert = [NSAlert new];
    alert.alertStyle = NSAlertStyleWarning;
    alert.messageText = t;
    alert.informativeText = m;
    // First button is the default (Return): keep it the safe one.
    NSButton *cancel = [alert addButtonWithTitle:@"Cancelar"];
    cancel.keyEquivalent = @"\r";
    NSButton *ok = [alert addButtonWithTitle:b];
    ok.keyEquivalent = @"";
    ok.hasDestructiveAction = YES;
    resp = [alert runModal];
  };

  if ([NSThread isMainThread]) {
    show();
  } else {
    dispatch_sync(dispatch_get_main_queue(), show);
  }
  return resp == NSAlertSecondButtonReturn;
}
