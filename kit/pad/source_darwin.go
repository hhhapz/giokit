//go:build darwin && !ios && cgo

package pad

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework GameController -framework Foundation

#import <Foundation/Foundation.h>
#import <GameController/GameController.h>
#include <pthread.h>
#include <string.h>

#define GIOKIT_PAD_MAX 4

typedef struct {
	int id;
	int family; // 0 generic, 1 xbox, 2 playstation, 3 nintendo
	unsigned int buttons; // bit per pad.Button
	float axes[6];
	char name[128];
} giokit_pad;

extern void giokitPadWake(void);

// The GameController framework calls back on the main queue; snapshots
// copy every pad's state there, and Poll reads the copy under the lock.
static pthread_mutex_t giokitPadMu = PTHREAD_MUTEX_INITIALIZER;
static giokit_pad giokitPadState[GIOKIT_PAD_MAX];
static int giokitPadCount;
static NSMutableArray<GCController *> *giokitPadControllers;
static NSMapTable<GCController *, NSNumber *> *giokitPadIDs;
static int giokitPadNextID;

// Button bits, in pad.Button order.
enum { B_SOUTH, B_EAST, B_WEST, B_NORTH, B_LS, B_RS, B_LT, B_RT, B_BACK, B_START, B_LSTICK, B_RSTICK, B_UP, B_DOWN, B_LEFT, B_RIGHT, B_GUIDE };

static unsigned int giokitPadBit(GCControllerButtonInput *b, int bit) {
	return b != nil && b.pressed ? 1u << bit : 0;
}

static int giokitPadFamily(GCController *c) {
	NSString *cat = @"";
	if (@available(macOS 10.15, *)) {
		cat = c.productCategory ?: @"";
	}
	NSString *s = [[cat stringByAppendingString:(c.vendorName ?: @"")] lowercaseString];
	if ([s containsString:@"xbox"]) return 1;
	if ([s containsString:@"dualshock"] || [s containsString:@"dualsense"]) return 2;
	if ([s containsString:@"switch"] || [s containsString:@"joy-con"] || [s containsString:@"nintendo"]) return 3;
	return 0;
}

static void giokitPadFill(giokit_pad *p, GCController *c, GCExtendedGamepad *g) {
	memset(p, 0, sizeof(*p));
	p->id = [[giokitPadIDs objectForKey:c] intValue];
	p->family = giokitPadFamily(c);
	strlcpy(p->name, (c.vendorName ?: @"Controller").UTF8String, sizeof(p->name));
	unsigned int b = 0;
	b |= giokitPadBit(g.buttonA, B_SOUTH) | giokitPadBit(g.buttonB, B_EAST);
	b |= giokitPadBit(g.buttonX, B_WEST) | giokitPadBit(g.buttonY, B_NORTH);
	b |= giokitPadBit(g.leftShoulder, B_LS) | giokitPadBit(g.rightShoulder, B_RS);
	b |= giokitPadBit(g.leftTrigger, B_LT) | giokitPadBit(g.rightTrigger, B_RT);
	b |= giokitPadBit(g.dpad.up, B_UP) | giokitPadBit(g.dpad.down, B_DOWN);
	b |= giokitPadBit(g.dpad.left, B_LEFT) | giokitPadBit(g.dpad.right, B_RIGHT);
	b |= giokitPadBit(g.buttonMenu, B_START);
	if (@available(macOS 10.15, *)) {
		b |= giokitPadBit(g.buttonOptions, B_BACK);
	}
	if (@available(macOS 10.14.1, *)) {
		b |= giokitPadBit(g.leftThumbstickButton, B_LSTICK) | giokitPadBit(g.rightThumbstickButton, B_RSTICK);
	}
	if (@available(macOS 11.0, *)) {
		b |= giokitPadBit(g.buttonHome, B_GUIDE);
	}
	p->buttons = b;
	p->axes[0] = g.leftThumbstick.xAxis.value;
	p->axes[1] = -g.leftThumbstick.yAxis.value; // GameController's Y points up
	p->axes[2] = g.rightThumbstick.xAxis.value;
	p->axes[3] = -g.rightThumbstick.yAxis.value;
	p->axes[4] = g.leftTrigger.value;
	p->axes[5] = g.rightTrigger.value;
}

static void giokitPadSnapshot(void) {
	pthread_mutex_lock(&giokitPadMu);
	giokitPadCount = 0;
	for (GCController *c in giokitPadControllers) {
		GCExtendedGamepad *g = c.extendedGamepad;
		if (g == nil || giokitPadCount == GIOKIT_PAD_MAX) {
			continue;
		}
		giokitPadFill(&giokitPadState[giokitPadCount++], c, g);
	}
	pthread_mutex_unlock(&giokitPadMu);
	giokitPadWake();
}

static void giokitPadAttach(GCController *c) {
	if (c.extendedGamepad == nil || [giokitPadControllers containsObject:c]) {
		return;
	}
	[giokitPadControllers addObject:c];
	[giokitPadIDs setObject:@(++giokitPadNextID) forKey:c];
	c.extendedGamepad.valueChangedHandler = ^(GCExtendedGamepad *g, GCControllerElement *e) {
		giokitPadSnapshot();
	};
	giokitPadSnapshot();
}

static void giokitPadStart(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		if (giokitPadControllers != nil) {
			return;
		}
		giokitPadControllers = [NSMutableArray array];
		giokitPadIDs = [NSMapTable weakToStrongObjectsMapTable];
		NSNotificationCenter *nc = [NSNotificationCenter defaultCenter];
		[nc addObserverForName:GCControllerDidConnectNotification object:nil queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *n) { giokitPadAttach(n.object); }];
		[nc addObserverForName:GCControllerDidDisconnectNotification object:nil queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *n) {
				[giokitPadControllers removeObject:n.object];
				giokitPadSnapshot();
			}];
		for (GCController *c in [GCController controllers]) {
			giokitPadAttach(c);
		}
	});
}

static int giokitPadPoll(giokit_pad *out, int max) {
	pthread_mutex_lock(&giokitPadMu);
	int n = giokitPadCount < max ? giokitPadCount : max;
	memcpy(out, giokitPadState, n * sizeof(giokit_pad));
	pthread_mutex_unlock(&giokitPadMu);
	return n;
}
*/
import "C"

import (
	"sync/atomic"
)

// The macOS source uses the GameController framework, which handles Xbox,
// PlayStation, Nintendo and MFi pads with their own layouts. It reports
// changes on the main queue through value-changed handlers; the source
// keeps a copy that Poll reads. There is one framework-wide set of pads,
// so every source shares it.

func platformSource() Source { return &gcSource{} }

var gcWake atomic.Pointer[func()]

//export giokitPadWake
func giokitPadWake() {
	if w := gcWake.Load(); w != nil {
		(*w)()
	}
}

type gcSource struct{}

func (s *gcSource) Start(wake func()) error {
	gcWake.Store(&wake)
	C.giokitPadStart()
	return nil
}

func (s *gcSource) Poll(dst []Pad) []Pad {
	var buf [C.GIOKIT_PAD_MAX]C.giokit_pad
	n := int(C.giokitPadPoll(&buf[0], C.GIOKIT_PAD_MAX))
	for _, c := range buf[:n] {
		p := Pad{ID: int(c.id), Name: C.GoString(&c.name[0]), Buttons: Buttons(c.buttons)}
		p.Family = [...]Family{Generic, Xbox, PlayStation, Nintendo}[c.family]
		for i := range AxisCount {
			p.Axes[i] = float32(c.axes[i])
		}
		dst = append(dst, p)
	}
	return dst
}

func (s *gcSource) Close() error {
	gcWake.Store(nil)
	return nil
}
