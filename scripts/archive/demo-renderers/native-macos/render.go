//go:build ignore

package main

/*
#cgo darwin LDFLAGS: -framework CoreFoundation -framework CoreGraphics -framework CoreText -framework ImageIO

#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <CoreText/CoreText.h>
#include <ImageIO/ImageIO.h>
#include <math.h>
#include <stdlib.h>

typedef struct {
	int width;
	int height;
	size_t bytes_per_row;
	void *pixels;
	CGColorSpaceRef color_space;
	CGContextRef context;
} NativeCanvas;

static NativeCanvas *canvas_create(int width, int height) {
	NativeCanvas *canvas = calloc(1, sizeof(NativeCanvas));
	if (canvas == NULL) return NULL;
	canvas->width = width;
	canvas->height = height;
	canvas->bytes_per_row = (size_t)width * 4;
	canvas->pixels = calloc((size_t)height, canvas->bytes_per_row);
	canvas->color_space = CGColorSpaceCreateWithName(kCGColorSpaceSRGB);
	if (canvas->pixels == NULL || canvas->color_space == NULL) goto fail;
	canvas->context = CGBitmapContextCreate(
		canvas->pixels, width, height, 8, canvas->bytes_per_row,
		canvas->color_space,
		kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big
	);
	if (canvas->context == NULL) goto fail;
	CGContextSetShouldAntialias(canvas->context, true);
	CGContextSetShouldSmoothFonts(canvas->context, true);
	return canvas;

fail:
	if (canvas->context != NULL) CGContextRelease(canvas->context);
	if (canvas->color_space != NULL) CGColorSpaceRelease(canvas->color_space);
	free(canvas->pixels);
	free(canvas);
	return NULL;
}

static void canvas_destroy(NativeCanvas *canvas) {
	if (canvas == NULL) return;
	if (canvas->context != NULL) CGContextRelease(canvas->context);
	if (canvas->color_space != NULL) CGColorSpaceRelease(canvas->color_space);
	free(canvas->pixels);
	free(canvas);
}

static CGFloat canvas_y(NativeCanvas *canvas, double top, double height) {
	return (CGFloat)canvas->height - (CGFloat)top - (CGFloat)height;
}

static void canvas_fill_rect(NativeCanvas *canvas, double x, double y, double width, double height,
		double red, double green, double blue, double alpha) {
	CGContextSetRGBFillColor(canvas->context, red, green, blue, alpha);
	CGContextFillRect(canvas->context, CGRectMake(x, canvas_y(canvas, y, height), width, height));
}

static void canvas_fill_round_rect(NativeCanvas *canvas, double x, double y, double width, double height,
		double radius, double red, double green, double blue, double alpha) {
	CGRect rect = CGRectMake(x, canvas_y(canvas, y, height), width, height);
	CGPathRef path = CGPathCreateWithRoundedRect(rect, radius, radius, NULL);
	CGContextSetRGBFillColor(canvas->context, red, green, blue, alpha);
	CGContextAddPath(canvas->context, path);
	CGContextFillPath(canvas->context);
	CGPathRelease(path);
}

static void canvas_stroke_round_rect(NativeCanvas *canvas, double x, double y, double width, double height,
		double radius, double line_width, double red, double green, double blue, double alpha) {
	CGRect rect = CGRectInset(CGRectMake(x, canvas_y(canvas, y, height), width, height), line_width / 2, line_width / 2);
	CGPathRef path = CGPathCreateWithRoundedRect(rect, radius, radius, NULL);
	CGContextSetRGBStrokeColor(canvas->context, red, green, blue, alpha);
	CGContextSetLineWidth(canvas->context, line_width);
	CGContextAddPath(canvas->context, path);
	CGContextStrokePath(canvas->context);
	CGPathRelease(path);
}

static void canvas_fill_circle(NativeCanvas *canvas, double center_x, double center_y, double radius,
		double red, double green, double blue, double alpha) {
	CGRect rect = CGRectMake(center_x - radius, canvas_y(canvas, center_y + radius, radius * 2), radius * 2, radius * 2);
	CGContextSetRGBFillColor(canvas->context, red, green, blue, alpha);
	CGContextFillEllipseInRect(canvas->context, rect);
}

static void canvas_linear_gradient(NativeCanvas *canvas,
		double x0, double y0, double x1, double y1,
		double r0, double g0, double b0,
		double r1, double g1, double b1) {
	CGFloat components[] = {r0, g0, b0, 1, r1, g1, b1, 1};
	CGFloat locations[] = {0, 1};
	CGGradientRef gradient = CGGradientCreateWithColorComponents(canvas->color_space, components, locations, 2);
	CGPoint start = CGPointMake(x0, (CGFloat)canvas->height - y0);
	CGPoint end = CGPointMake(x1, (CGFloat)canvas->height - y1);
	CGContextDrawLinearGradient(canvas->context, gradient, start, end,
		kCGGradientDrawsBeforeStartLocation | kCGGradientDrawsAfterEndLocation);
	CGGradientRelease(gradient);
}

static CTLineRef canvas_make_line(const char *text, const char *font_name, double font_size,
		double red, double green, double blue, double alpha) {
	CFStringRef string = CFStringCreateWithCString(NULL, text, kCFStringEncodingUTF8);
	CFStringRef name = CFStringCreateWithCString(NULL, font_name, kCFStringEncodingUTF8);
	if (string == NULL || name == NULL) {
		if (string != NULL) CFRelease(string);
		if (name != NULL) CFRelease(name);
		return NULL;
	}
	CTFontRef font = CTFontCreateWithName(name, font_size, NULL);
	CGColorRef color = CGColorCreateGenericRGB(red, green, blue, alpha);
	const void *keys[] = {kCTFontAttributeName, kCTForegroundColorAttributeName};
	const void *values[] = {font, color};
	CFDictionaryRef attributes = CFDictionaryCreate(NULL, keys, values, 2,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFAttributedStringRef attributed = CFAttributedStringCreate(NULL, string, attributes);
	CTLineRef line = CTLineCreateWithAttributedString(attributed);
	CFRelease(attributed);
	CFRelease(attributes);
	CGColorRelease(color);
	CFRelease(font);
	CFRelease(name);
	CFRelease(string);
	return line;
}

static double canvas_text_width(const char *text, const char *font_name, double font_size) {
	CTLineRef line = canvas_make_line(text, font_name, font_size, 1, 1, 1, 1);
	if (line == NULL) return 0;
	double width = CTLineGetTypographicBounds(line, NULL, NULL, NULL);
	CFRelease(line);
	return width;
}

static void canvas_draw_text(NativeCanvas *canvas, const char *text, const char *font_name, double font_size,
		double x, double baseline, double scale_x,
		double red, double green, double blue, double alpha) {
	CTLineRef line = canvas_make_line(text, font_name, font_size, red, green, blue, alpha);
	if (line == NULL) return;
	CGContextSaveGState(canvas->context);
	CGContextTranslateCTM(canvas->context, x, (CGFloat)canvas->height - baseline);
	CGContextScaleCTM(canvas->context, scale_x, 1);
	CGContextSetTextMatrix(canvas->context, CGAffineTransformIdentity);
	CGContextSetTextPosition(canvas->context, 0, 0);
	CTLineDraw(line, canvas->context);
	CGContextRestoreGState(canvas->context);
	CFRelease(line);
}

static int canvas_write_png(NativeCanvas *canvas, const char *path) {
	CGImageRef image = CGBitmapContextCreateImage(canvas->context);
	CFStringRef path_string = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
	CFURLRef url = CFURLCreateWithFileSystemPath(NULL, path_string, kCFURLPOSIXPathStyle, false);
	CGImageDestinationRef destination = CGImageDestinationCreateWithURL(url, CFSTR("public.png"), 1, NULL);
	int ok = 0;
	if (destination != NULL) {
		CGImageDestinationAddImage(destination, image, NULL);
		ok = CGImageDestinationFinalize(destination) ? 1 : 0;
		CFRelease(destination);
	}
	CFRelease(url);
	CFRelease(path_string);
	CGImageRelease(image);
	return ok;
}
*/
import "C"

import (
	"fmt"
	"image"
	imagecolor "image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/wnma3mz/yuxin/internal/app"
)

const (
	canvasWidth  = 1440
	canvasHeight = 900
	monoFont     = "Menlo-Regular"
	cjkFont      = "PingFangSC-Regular"
)

type color struct {
	r, g, b, a float64
}

type nativeCanvas struct {
	ptr *C.NativeCanvas
}

func newNativeCanvas() (*nativeCanvas, error) {
	ptr := C.canvas_create(canvasWidth, canvasHeight)
	if ptr == nil {
		return nil, fmt.Errorf("创建原生画布失败")
	}
	return &nativeCanvas{ptr: ptr}, nil
}

func (c *nativeCanvas) close() {
	C.canvas_destroy(c.ptr)
}

func (c *nativeCanvas) fillRect(x, y, width, height float64, fill color) {
	C.canvas_fill_rect(c.ptr, C.double(x), C.double(y), C.double(width), C.double(height),
		C.double(fill.r), C.double(fill.g), C.double(fill.b), C.double(fill.a))
}

func (c *nativeCanvas) fillRoundRect(x, y, width, height, radius float64, fill color) {
	C.canvas_fill_round_rect(c.ptr, C.double(x), C.double(y), C.double(width), C.double(height), C.double(radius),
		C.double(fill.r), C.double(fill.g), C.double(fill.b), C.double(fill.a))
}

func (c *nativeCanvas) strokeRoundRect(x, y, width, height, radius, lineWidth float64, stroke color) {
	C.canvas_stroke_round_rect(c.ptr, C.double(x), C.double(y), C.double(width), C.double(height),
		C.double(radius), C.double(lineWidth), C.double(stroke.r), C.double(stroke.g), C.double(stroke.b), C.double(stroke.a))
}

func (c *nativeCanvas) fillCircle(x, y, radius float64, fill color) {
	C.canvas_fill_circle(c.ptr, C.double(x), C.double(y), C.double(radius),
		C.double(fill.r), C.double(fill.g), C.double(fill.b), C.double(fill.a))
}

func (c *nativeCanvas) gradient(start, end color) {
	C.canvas_linear_gradient(c.ptr, 0, 0, canvasWidth, canvasHeight,
		C.double(start.r), C.double(start.g), C.double(start.b),
		C.double(end.r), C.double(end.g), C.double(end.b))
}

func textWidth(text, font string, size float64) float64 {
	cText := C.CString(text)
	cFont := C.CString(font)
	defer C.free(unsafe.Pointer(cText))
	defer C.free(unsafe.Pointer(cFont))
	return float64(C.canvas_text_width(cText, cFont, C.double(size)))
}

func (c *nativeCanvas) drawText(text, font string, size, x, baseline, scaleX float64, fill color) {
	cText := C.CString(text)
	cFont := C.CString(font)
	defer C.free(unsafe.Pointer(cText))
	defer C.free(unsafe.Pointer(cFont))
	C.canvas_draw_text(c.ptr, cText, cFont, C.double(size), C.double(x), C.double(baseline), C.double(scaleX),
		C.double(fill.r), C.double(fill.g), C.double(fill.b), C.double(fill.a))
}

func (c *nativeCanvas) writePNG(path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	if C.canvas_write_png(c.ptr, cPath) == 0 {
		return fmt.Errorf("写入 PNG 失败：%s", path)
	}
	return nil
}

func rgb(hex uint32) color {
	return color{
		r: float64((hex>>16)&0xff) / 255,
		g: float64((hex>>8)&0xff) / 255,
		b: float64(hex&0xff) / 255,
		a: 1,
	}
}

func withAlpha(value color, alpha float64) color {
	value.a = alpha
	return value
}

func drawBackground(c *nativeCanvas) {
	c.gradient(rgb(0x07101f), rgb(0x101827))
	for i := 0; i < 8; i++ {
		c.fillRoundRect(50-float64(i), 38+float64(i*2), 1340+float64(i*2), 824, 18,
			withAlpha(rgb(0x000000), 0.025))
	}
}

func drawTerminal(c *nativeCanvas, title string) {
	c.fillRoundRect(50, 38, 1340, 824, 18, rgb(0x0a1220))
	c.strokeRoundRect(50, 38, 1340, 824, 18, 1, rgb(0x263448))
	c.fillRoundRect(50, 38, 1340, 50, 18, rgb(0x111c2d))
	c.fillRect(50, 70, 1340, 18, rgb(0x111c2d))
	for _, dot := range []struct {
		x     float64
		color color
	}{{78, rgb(0xff5f57)}, {102, rgb(0xfebc2e)}, {126, rgb(0x28c840)}} {
		c.fillCircle(dot.x, 63, 6, dot.color)
	}
	const size = 14
	x := (canvasWidth - textWidth(title, monoFont, size)) / 2
	c.drawText(title, monoFont, size, x, 68, 1, rgb(0x9aa8bc))
}

func isSpecialRune(r rune) bool {
	return strings.ContainsRune("╭╮╰╯├┤│─━◆●○█░✓", r)
}

func isWideRune(r rune) bool {
	return r > 255 && !isSpecialRune(r)
}

func displayWidth(line string) int {
	width := 0
	for _, r := range line {
		if isWideRune(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

func runeColor(r rune, line string) color {
	if strings.ContainsRune("╭╮╰╯├┤│─", r) {
		return rgb(0x53647a)
	}
	switch r {
	case '█':
		return rgb(0x34d399)
	case '░':
		return rgb(0x334155)
	case '◆':
		return rgb(0xfbbf24)
	case '●', '━':
		return rgb(0x22d3ee)
	case '○':
		return rgb(0x64748b)
	}
	trimmed := strings.TrimSpace(line)
	if strings.Contains(line, "今日入账") && strings.HasPrefix(trimmed, "│") && strings.Contains(line, "¥") {
		return rgb(0xfbbf24)
	}
	if strings.Contains(line, "演示模式") {
		return rgb(0xf8fafc)
	}
	if strings.Contains(line, "正在上班") {
		return rgb(0x4ade80)
	}
	if strings.Contains(line, "[e] 配置") {
		return rgb(0x94a3b8)
	}
	return rgb(0xdbe5f2)
}

func drawTerminalText(c *nativeCanvas, text string, fontSize, lineHeight, startY float64) {
	lines := strings.Split(text, "\n")
	maxColumns := 0
	for _, line := range lines {
		maxColumns = max(maxColumns, displayWidth(line))
	}
	cell := textWidth("M", monoFont, fontSize)
	startX := (canvasWidth - float64(maxColumns)*cell) / 2
	for row, line := range lines {
		column := 0
		for _, r := range line {
			cells := 1
			font := monoFont
			if isWideRune(r) {
				cells = 2
				font = cjkFont
			}
			text := string(r)
			measured := math.Max(1, textWidth(text, font, fontSize))
			scale := math.Min(1.22, float64(cells)*cell/measured)
			c.drawText(text, font, fontSize, startX+float64(column)*cell,
				startY+float64(row)*lineHeight, scale, runeColor(r, line))
			column += cells
		}
	}
}

func drawCentered(c *nativeCanvas, text, font string, size, baseline float64, fill color) {
	x := (canvasWidth - textWidth(text, font, size)) / 2
	c.drawText(text, font, size, x, baseline, 1, fill)
}

func renderIntro(path string) error {
	c, err := newNativeCanvas()
	if err != nil {
		return err
	}
	defer c.close()
	drawBackground(c)
	drawTerminal(c, "yuxin — native Go renderer")
	drawCentered(c, "YUXIN", "Menlo-Bold", 74, 330, rgb(0xf8fafc))
	drawCentered(c, "余薪", "PingFangSC-Semibold", 30, 385, rgb(0xdbe5f2))
	drawCentered(c, "摸鱼有数，下班有期。", "PingFangSC-Medium", 30, 475, rgb(0x4ade80))
	c.fillRoundRect(canvasWidth/2-245, 535, 490, 54, 27, rgb(0x111f31))
	c.strokeRoundRect(canvasWidth/2-245, 535, 490, 54, 27, 1, rgb(0x2b3b51))
	drawCentered(c, "无账号  ·  离线运行  ·  数据只在本地", cjkFont, 19, 570, rgb(0x9fb0c5))
	return c.writePNG(path)
}

func renderDashboard(path, dashboard string) error {
	c, err := newNativeCanvas()
	if err != nil {
		return err
	}
	defer c.close()
	drawBackground(c)
	drawTerminal(c, "yuxin — 演示模式 · Go 原生渲染")
	drawTerminalText(c, dashboard, 16.7, 27.5, 116)
	return c.writePNG(path)
}

func renderShare(path, share string) error {
	c, err := newNativeCanvas()
	if err != nil {
		return err
	}
	defer c.close()
	drawBackground(c)
	drawTerminal(c, "yuxin share — 演示数据 · Go 原生渲染")
	c.drawText("一键生成可分享画面", "PingFangSC-Semibold", 26, 390, 155, 1, rgb(0xf8fafc))
	c.drawText("固定合成数据，不暴露工资、存款或退休信息", cjkFont, 18, 390, 192, 1, rgb(0x8fa1b7))
	drawTerminalText(c, share, 19, 32, 260)
	return c.writePNG(path)
}

func readPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

func blendFrames(from, to image.Image, progress float64) *image.RGBA {
	bounds := from.Bounds().Intersect(to.Bounds())
	frame := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			fromR, fromG, fromB, _ := from.At(x, y).RGBA()
			toR, toG, toB, _ := to.At(x, y).RGBA()
			mix := func(a, b uint32) uint8 {
				value := float64(a)*(1-progress) + float64(b)*progress
				return uint8(math.Round(value / 257))
			}
			frame.SetRGBA(x, y, imagecolor.RGBA{
				R: mix(fromR, toR),
				G: mix(fromG, toG),
				B: mix(fromB, toB),
				A: 0xff,
			})
		}
	}
	return frame
}

func palettedFrame(source image.Image) *image.Paletted {
	frame := image.NewPaletted(source.Bounds(), palette.Plan9)
	draw.FloydSteinberg.Draw(frame, frame.Rect, source, source.Bounds().Min)
	return frame
}

func writeAnimatedGIF(path string, framePaths []string) error {
	frames := make([]image.Image, 0, len(framePaths))
	for _, framePath := range framePaths {
		frame, err := readPNG(framePath)
		if err != nil {
			return fmt.Errorf("读取动画帧：%w", err)
		}
		frames = append(frames, frame)
	}
	if len(frames) == 0 {
		return fmt.Errorf("没有可用的动画帧")
	}

	animation := &gif.GIF{LoopCount: 0}
	appendFrame := func(frame image.Image, delay int) {
		animation.Image = append(animation.Image, palettedFrame(frame))
		animation.Delay = append(animation.Delay, delay)
		animation.Disposal = append(animation.Disposal, gif.DisposalNone)
	}
	appendFrame(frames[0], 500)
	for scene := 0; scene < len(frames)-1; scene++ {
		for step := 1; step <= 9; step++ {
			appendFrame(blendFrames(frames[scene], frames[scene+1], float64(step)/10), 5)
		}
		hold := 535
		if scene == len(frames)-2 {
			hold = 340
		}
		appendFrame(frames[scene+1], hold)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := gif.EncodeAll(file, animation); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/render-demo-native.go OUTPUT_DIR")
		os.Exit(2)
	}
	if !utf8.ValidString(os.Args[1]) {
		fmt.Fprintln(os.Stderr, "输出目录不是有效的 UTF-8 路径")
		os.Exit(2)
	}

	snapshot, config, err := app.DemoDashboard()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	share, err := app.RenderShareCard(snapshot, config, "overview")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dashboard := app.RenderDashboard(snapshot, config, 110, false)

	output := os.Args[1]
	if err := os.MkdirAll(output, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	frames := []struct {
		name   string
		render func(string) error
	}{
		{"intro.png", renderIntro},
		{"dashboard.png", func(path string) error { return renderDashboard(path, dashboard) }},
		{"share.png", func(path string) error { return renderShare(path, share) }},
	}
	for _, frame := range frames {
		path := filepath.Join(output, frame.name)
		if err := frame.render(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(path)
	}
	animationPath := filepath.Join(output, "yuxin-demo-native.gif")
	framePaths := []string{
		filepath.Join(output, "dashboard.png"),
		filepath.Join(output, "share.png"),
	}
	if err := writeAnimatedGIF(animationPath, framePaths); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(animationPath)
}
