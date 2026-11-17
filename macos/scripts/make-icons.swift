#!/usr/bin/env swift
//
// Renders the desktop app's icons into macos/DocketApp/Resources/Assets.xcassets:
//
//   AppIcon.appiconset     the full-color Dock/Finder icon, every mac size
//   StatusIcon.imageset    the monochrome menu-bar template image
//
// The PNGs are committed, so this runs only when the artwork changes:
//
//   swift macos/scripts/make-icons.swift
//   swift macos/scripts/make-icons.swift --ios   # iPhone icon and launch artwork
//
// Everything is drawn from code (no source image), on the macOS icon grid: an
// 824-point rounded square centered in a 1024 canvas. The glyph is a shell
// prompt, `dkt`, and an underline cursor: the app is a terminal that runs dkt. Sizes
// of 32 pixels and under drop the prompt and cursor, which blur at that scale.

import AppKit
import CoreText

let scriptURL = URL(fileURLWithPath: CommandLine.arguments[0]).standardizedFileURL
let macosDir = scriptURL.deletingLastPathComponent().deletingLastPathComponent()
let assets = macosDir.appendingPathComponent("DocketApp/Resources/Assets.xcassets")

// MARK: - Drawing helpers

func rgb(_ hex: UInt32, _ alpha: CGFloat = 1) -> CGColor {
    CGColor(
        srgbRed: CGFloat((hex >> 16) & 0xFF) / 255,
        green: CGFloat((hex >> 8) & 0xFF) / 255,
        blue: CGFloat(hex & 0xFF) / 255,
        alpha: alpha)
}

func bitmap(_ pixels: Int, opaque: Bool = false, _ draw: (CGContext) -> Void) -> Data {
    let space = CGColorSpace(name: CGColorSpace.sRGB)!
    let ctx = CGContext(
        data: nil, width: pixels, height: pixels, bitsPerComponent: 8, bytesPerRow: 0,
        space: space, bitmapInfo: (opaque ? CGImageAlphaInfo.noneSkipLast : .premultipliedLast).rawValue)!
    ctx.interpolationQuality = .high
    ctx.setShouldAntialias(true)
    draw(ctx)
    let rep = NSBitmapImageRep(cgImage: ctx.makeImage()!)
    return rep.representation(using: .png, properties: [:])!
}

/// A monospaced line of text as a path, so it can be filled, measured, and
/// centered without a text layout pass.
func textPath(_ text: String, size: CGFloat, weight: NSFont.Weight) -> CGPath {
    let font = NSFont.monospacedSystemFont(ofSize: size, weight: weight) as CTFont
    let path = CGMutablePath()
    var x: CGFloat = 0
    for scalar in text.unicodeScalars {
        var char = UniChar(scalar.value)
        var glyph = CGGlyph()
        CTFontGetGlyphsForCharacters(font, &char, &glyph, 1)
        if let outline = CTFontCreatePathForGlyph(font, glyph, nil) {
            path.addPath(outline, transform: CGAffineTransform(translationX: x, y: 0))
        }
        var advance = CGSize.zero
        CTFontGetAdvancesForGlyphs(font, .horizontal, &glyph, &advance, 1)
        x += advance.width
    }
    return path
}

func centered(_ path: CGPath, in rect: CGRect, dx: CGFloat = 0, dy: CGFloat = 0) -> CGPath {
    let box = path.boundingBoxOfPath
    var move = CGAffineTransform(
        translationX: rect.midX - box.midX + dx, y: rect.midY - box.midY + dy)
    return path.copy(using: &move)!
}

// MARK: - App icon

/// Draws the app icon on a 1024 canvas scaled to `pixels`.
func appIcon(pixels: Int, mobile: Bool = false) -> Data {
    bitmap(pixels, opaque: mobile) { ctx in
        let scale = CGFloat(pixels) / 1024
        ctx.scaleBy(x: scale, y: scale)
        // iOS applies its own corner mask. Fill the canvas with the desktop
        // tile's artwork, without its transparent margin, rounding, or shadow.
        if mobile {
            ctx.scaleBy(x: 1024.0 / 824, y: 1024.0 / 824)
            ctx.translateBy(x: -100, y: -100)
        }
        let detailed = pixels > 32

        // Tile: 824 on the 1024 grid, corner radius 185, with a soft drop shadow.
        let tile = CGRect(x: 100, y: 100, width: 824, height: 824)
        let radius: CGFloat = mobile ? 0 : 185
        let shape = CGPath(roundedRect: tile, cornerWidth: radius, cornerHeight: radius, transform: nil)
        ctx.saveGState()
        if !mobile {
            ctx.setShadow(offset: CGSize(width: 0, height: -10), blur: 28, color: rgb(0x000000, 0.35))
        }
        ctx.addPath(shape)
        ctx.setFillColor(rgb(0x1B2030))
        ctx.fillPath()
        ctx.restoreGState()

        ctx.saveGState()
        ctx.addPath(shape)
        ctx.clip()
        let gradient = CGGradient(
            colorsSpace: CGColorSpace(name: CGColorSpace.sRGB),
            colors: [rgb(0x3A4460), rgb(0x1B2030)] as CFArray, locations: [0, 1])!
        ctx.drawLinearGradient(
            gradient, start: CGPoint(x: 512, y: 924), end: CGPoint(x: 512, y: 100), options: [])
        // A faint title-bar band with three window dots, read as "terminal".
        if detailed {
            ctx.setFillColor(rgb(0xFFFFFF, 0.06))
            ctx.fill(CGRect(x: 100, y: 800, width: 824, height: 124))
            for (i, color) in [0xFF6B6B, 0xFFC857, 0x7FD8A6].enumerated() {
                ctx.setFillColor(rgb(UInt32(color), 0.9))
                ctx.fillEllipse(in: CGRect(x: 186 + CGFloat(i) * 62, y: 843, width: 38, height: 38))
            }
        }
        ctx.restoreGState()

        // Hairline edge so the tile holds on light and dark backgrounds.
        if !mobile {
            ctx.addPath(shape)
            ctx.setStrokeColor(rgb(0xFFFFFF, 0.10))
            ctx.setLineWidth(4)
            ctx.strokePath()
        }

        let green = rgb(0xA6E3B8)
        let body = CGRect(x: 100, y: 100, width: 824, height: 700)
        if detailed {
            // `› dkt▌` on one baseline, centered in the area under the title band.
            let prompt = textPath("›", size: 190, weight: .bold)
            let word = textPath("dkt", size: 210, weight: .bold)
            let promptBox = prompt.boundingBoxOfPath
            let wordBox = word.boundingBoxOfPath
            let gap: CGFloat = 40
            let cursorWidth: CGFloat = 110
            let total = promptBox.width + gap + wordBox.width + gap + cursorWidth
            var x = body.midX - total / 2
            let baseline = body.midY - wordBox.height / 2 - wordBox.minY

            var t = CGAffineTransform(translationX: x - promptBox.minX, y: baseline + wordBox.minY - promptBox.minY + 20)
            ctx.addPath(prompt.copy(using: &t)!)
            ctx.setFillColor(rgb(0x7F8BAA))
            ctx.fillPath()
            x += promptBox.width + gap

            t = CGAffineTransform(translationX: x - wordBox.minX, y: baseline)
            ctx.addPath(word.copy(using: &t)!)
            ctx.setFillColor(green)
            ctx.fillPath()
            x += wordBox.width + gap

            ctx.setFillColor(rgb(0xA6E3B8, 0.75))
            ctx.fill(CGRect(x: x, y: baseline + wordBox.minY, width: cursorWidth, height: 44))
        } else {
            // Small sizes: a heavier `dkt` filling the tile.
            let word = centered(textPath("dkt", size: 340, weight: .heavy), in: tile)
            ctx.addPath(word)
            ctx.setFillColor(green)
            ctx.fillPath()
        }
    }
}

// MARK: - Status icon

/// The menu-bar template: `dkt` inside a rounded terminal frame, black on
/// clear. AppKit tints it for light, dark, and highlighted menu bars.
func statusIcon(pixels: Int) -> Data {
    bitmap(pixels) { ctx in
        let scale = CGFloat(pixels) / 18
        ctx.scaleBy(x: scale, y: scale)
        let frame = CGRect(x: 1.25, y: 2.25, width: 15.5, height: 13.5)
        ctx.addPath(CGPath(roundedRect: frame, cornerWidth: 3, cornerHeight: 3, transform: nil))
        ctx.setStrokeColor(rgb(0x000000))
        ctx.setLineWidth(1.5)
        ctx.strokePath()
        let word = centered(textPath("dkt", size: 7.5, weight: .bold), in: frame, dy: -0.25)
        ctx.addPath(word)
        ctx.setFillColor(rgb(0x000000))
        ctx.fillPath()
    }
}

// MARK: - Output

func write(_ data: Data, _ url: URL) {
    try! data.write(to: url, options: .atomic)
}

func writeJSON(_ object: Any, _ url: URL) {
    let data = try! JSONSerialization.data(
        withJSONObject: object, options: [.prettyPrinted, .sortedKeys])
    write(data + Data("\n".utf8), url)
}

let fm = FileManager.default

if CommandLine.arguments.dropFirst().contains("--ios") {
    let mobileAssets = macosDir.deletingLastPathComponent()
        .appendingPathComponent("ios/DocketMobile/Resources/Assets.xcassets")
    let iconDir = mobileAssets.appendingPathComponent("AppIcon.appiconset")
    let launchDir = mobileAssets.appendingPathComponent("LaunchIcon.imageset")
    for dir in [iconDir, launchDir] {
        try! fm.createDirectory(at: dir, withIntermediateDirectories: true)
    }
    let info: [String: Any] = ["author": "xcode", "version": 1]
    writeJSON(["info": info], mobileAssets.appendingPathComponent("Contents.json"))
    write(appIcon(pixels: 1024, mobile: true), iconDir.appendingPathComponent("AppIcon.png"))
    writeJSON([
        "images": [["filename": "AppIcon.png", "idiom": "universal", "platform": "ios", "size": "1024x1024"]],
        "info": info,
    ], iconDir.appendingPathComponent("Contents.json"))
    var launchImages: [[String: String]] = []
    for scale in [1, 2, 3] {
        let name = "LaunchIcon@\(scale)x.png"
        write(appIcon(pixels: 128 * scale), launchDir.appendingPathComponent(name))
        launchImages.append(["filename": name, "idiom": "universal", "scale": "\(scale)x"])
    }
    writeJSON(["images": launchImages, "info": info], launchDir.appendingPathComponent("Contents.json"))
    print("Wrote \(iconDir.path) and \(launchDir.path)")
    exit(0)
}

let appIconDir = assets.appendingPathComponent("AppIcon.appiconset")
try! fm.createDirectory(at: appIconDir, withIntermediateDirectories: true)
var appImages: [[String: String]] = []
for points in [16, 32, 128, 256, 512] {
    for scale in [1, 2] {
        let name = scale == 1 ? "icon_\(points)x\(points).png" : "icon_\(points)x\(points)@2x.png"
        write(appIcon(pixels: points * scale), appIconDir.appendingPathComponent(name))
        appImages.append([
            "filename": name, "idiom": "mac", "scale": "\(scale)x", "size": "\(points)x\(points)",
        ])
    }
}
writeJSON(
    ["images": appImages, "info": ["author": "xcode", "version": 1]],
    appIconDir.appendingPathComponent("Contents.json"))

let statusDir = assets.appendingPathComponent("StatusIcon.imageset")
try! fm.createDirectory(at: statusDir, withIntermediateDirectories: true)
var statusImages: [[String: String]] = []
for scale in [1, 2] {
    let name = scale == 1 ? "status_18.png" : "status_18@2x.png"
    write(statusIcon(pixels: 18 * scale), statusDir.appendingPathComponent(name))
    statusImages.append(["filename": name, "idiom": "mac", "scale": "\(scale)x"])
}
writeJSON(
    [
        "images": statusImages, "info": ["author": "xcode", "version": 1],
        "properties": ["template-rendering-intent": "template"],
    ],
    statusDir.appendingPathComponent("Contents.json"))

print("Wrote \(appIconDir.path) and \(statusDir.path)")
