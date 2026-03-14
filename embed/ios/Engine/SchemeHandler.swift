import Foundation
import WebKit
import UniformTypeIdentifiers

/// Handles `phptoro://` URL scheme requests for the WKWebView.
///
/// - `phptoro://localhost/...` maps to files in `ScreenViewController.webDir`
/// - `phptoro://file/...` serves any file from the app sandbox (e.g. camera photos saved elsewhere)
final class SchemeHandler: NSObject, WKURLSchemeHandler {
    private static let mimeTypes: [String: String] = [
        "html": "text/html",
        "htm":  "text/html",
        "css":  "text/css",
        "js":   "application/javascript",
        "json": "application/json",
        "jpg":  "image/jpeg",
        "jpeg": "image/jpeg",
        "png":  "image/png",
        "gif":  "image/gif",
        "svg":  "image/svg+xml",
        "webp": "image/webp",
        "ico":  "image/x-icon",
        "woff": "font/woff",
        "woff2": "font/woff2",
        "ttf":  "font/ttf",
        "otf":  "font/otf",
        "mp4":  "video/mp4",
        "webm": "video/webm",
        "mp3":  "audio/mpeg",
        "wav":  "audio/wav",
        "pdf":  "application/pdf",
        "xml":  "application/xml",
        "txt":  "text/plain",
    ]

    func webView(_ webView: WKWebView, start urlSchemeTask: any WKURLSchemeTask) {
        guard let url = urlSchemeTask.request.url else {
            urlSchemeTask.didFailWithError(NSError(domain: "SchemeHandler", code: -1, userInfo: [NSLocalizedDescriptionKey: "No URL"]))
            return
        }

        let host = url.host ?? ""
        let path = url.path

        let fileURL: URL

        if host == "file" {
            // phptoro://file/<absolute-path> — serve any sandbox file
            fileURL = URL(fileURLWithPath: path)
        } else {
            // phptoro://localhost/... — map to webDir
            let relativePath = String(path.dropFirst()) // remove leading /
            if relativePath.isEmpty {
                urlSchemeTask.didFailWithError(NSError(domain: "SchemeHandler", code: -2, userInfo: [NSLocalizedDescriptionKey: "Empty path"]))
                return
            }
            fileURL = ScreenViewController.webDir.appendingPathComponent(relativePath)
        }

        guard FileManager.default.fileExists(atPath: fileURL.path) else {
            dbg.verbose("Scheme", "404: \(url.absoluteString) → \(fileURL.path)")
            urlSchemeTask.didFailWithError(NSError(domain: "SchemeHandler", code: -3, userInfo: [NSLocalizedDescriptionKey: "File not found: \(fileURL.path)"]))
            return
        }

        do {
            let data = try Data(contentsOf: fileURL, options: .mappedIfSafe)
            let ext = fileURL.pathExtension.lowercased()
            let mimeType = Self.mimeTypes[ext] ?? "application/octet-stream"

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: "HTTP/1.1",
                headerFields: [
                    "Content-Type": mimeType,
                    "Content-Length": "\(data.count)",
                    "Access-Control-Allow-Origin": "*",
                ]
            )!

            urlSchemeTask.didReceive(response)
            urlSchemeTask.didReceive(data)
            urlSchemeTask.didFinish()
        } catch {
            dbg.error("Scheme", "Failed to read \(fileURL.path): \(error)")
            urlSchemeTask.didFailWithError(error)
        }
    }

    func webView(_ webView: WKWebView, stop urlSchemeTask: any WKURLSchemeTask) {
        // Nothing to cancel — reads are synchronous
    }
}
