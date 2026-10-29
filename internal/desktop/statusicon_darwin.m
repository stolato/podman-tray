#import <AppKit/AppKit.h>

// The image is drawn on demand, so AppKit redraws it whenever the status item's
// appearance changes (dark mode, or a wallpaper that flips the menu bar's color).
void pt_set_badged_icon(const void *png, int len, double dot_x, double dot_y, double dot_r) {
  NSImage *mask = [[NSImage alloc] initWithData:[NSData dataWithBytes:png length:len]];
  if (mask == nil) {
    return;
  }
  NSSize size = NSMakeSize(16, 16);
  mask.size = size;

  NSImage *image = [NSImage imageWithSize:size
                                  flipped:NO
                           drawingHandler:^BOOL(NSRect rect) {
                             NSAppearanceName match = [NSAppearance.currentDrawingAppearance
                                 bestMatchFromAppearancesWithNames:@[ NSAppearanceNameAqua, NSAppearanceNameDarkAqua ]];
                             NSColor *ink = [match isEqualToString:NSAppearanceNameDarkAqua] ? NSColor.whiteColor
                                                                                             : NSColor.blackColor;

                             NSRect proposed = rect;
                             CGImageRef cg = [mask CGImageForProposedRect:&proposed
                                                                  context:NSGraphicsContext.currentContext
                                                                    hints:nil];
                             CGContextRef ctx = NSGraphicsContext.currentContext.CGContext;
                             CGContextSaveGState(ctx);
                             CGContextClipToMask(ctx, NSRectToCGRect(rect), cg);
                             [ink setFill];
                             NSRectFill(rect);
                             CGContextRestoreGState(ctx);

                             [NSColor.systemGreenColor setFill];
                             [[NSBezierPath
                                 bezierPathWithOvalInRect:NSMakeRect(dot_x - dot_r, dot_y - dot_r, 2 * dot_r, 2 * dot_r)]
                                 fill];
                             return YES;
                           }];

  // fyne.io/systray's app delegate owns the status item; reuse its setIcon:.
  dispatch_async(dispatch_get_main_queue(), ^{
    id delegate = NSApp.delegate;
    if ([delegate respondsToSelector:@selector(setIcon:)]) {
      [delegate performSelector:@selector(setIcon:) withObject:image];
    }
  });
}
