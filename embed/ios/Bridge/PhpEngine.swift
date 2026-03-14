import Foundation

/// PhpEngine wraps the C runtime (SAPI + extension).
/// Boots PHP, executes scripts, and wires up the native handler.
final class PhpEngine {
    static let shared = PhpEngine()

    private var initialized = false

    /// Boot PHP with the given data directory.
    func initialize(dataDir: String, documentRoot: String) {
        guard !initialized else { return }

        dbg.log("PhpEngine", "initialize(dataDir: \(dataDir), docRoot: \(documentRoot))")

        // Register the native handler BEFORE php_init
        phptoro_set_native_handler { ns, method, argsJson in
            guard let ns = ns, let method = method, let argsJson = argsJson else {
                return nil
            }
            let nsStr = String(cString: ns)
            let methodStr = String(cString: method)
            let argsStr = String(cString: argsJson)

            dbg.verbose("Bridge", "→ native_call(\(nsStr).\(methodStr)) args: \(argsStr.prefix(200))")

            let result = PluginHost.shared.dispatch(ns: nsStr, method: methodStr, argsJson: argsStr)

            dbg.verbose("Bridge", "← native_call(\(nsStr).\(methodStr)) result: \(result.prefix(200))")

            // Return a malloc'd C string (caller frees)
            guard let resultData = result.data(using: .utf8) else { return nil }
            let buf = malloc(resultData.count + 1)!.assumingMemoryBound(to: CChar.self)
            resultData.copyBytes(to: UnsafeMutableBufferPointer(start: buf, count: resultData.count))
            buf[resultData.count] = 0
            return buf
        }

        let result = phptoro_php_init(dataDir)
        guard result == 0 else {
            fatalError("phptoro_php_init failed with code \(result)")
        }

        dbg.log("PhpEngine", "PHP engine initialized")
        initialized = true
    }

    /// Execute a PHP script and return the parsed JSON response.
    func execute(scriptPath: String, documentRoot: String, body: [String: Any]? = nil) -> [String: Any]? {
        let bodyJson: Data?
        if let body = body {
            bodyJson = try? JSONSerialization.data(withJSONObject: body)
        } else {
            bodyJson = nil
        }

        var req = phptoro_request()
        let methodStr = "POST"
        let uriStr = "/index.php"
        let contentTypeStr = "application/json"

        return methodStr.withCString { methodPtr in
            uriStr.withCString { uriPtr in
                scriptPath.withCString { scriptPtr in
                    documentRoot.withCString { docRootPtr in
                        contentTypeStr.withCString { ctPtr in
                            req.method = methodPtr
                            req.uri = uriPtr
                            req.script_path = scriptPtr
                            req.document_root = docRootPtr
                            req.content_type = ctPtr
                            req.header_count = 0

                            if let bodyJson = bodyJson {
                                return bodyJson.withUnsafeBytes { rawBuf in
                                    let ptr = rawBuf.baseAddress?.assumingMemoryBound(to: UInt8.self)
                                    req.body = ptr
                                    req.body_len = bodyJson.count

                                    return executeRequest(&req)
                                }
                            } else {
                                req.body = nil
                                req.body_len = 0
                                return executeRequest(&req)
                            }
                        }
                    }
                }
            }
        }
    }

    private func executeRequest(_ req: inout phptoro_request) -> [String: Any]? {
        var resp = phptoro_response()
        let status = phptoro_php_execute(&req, &resp)

        defer { phptoro_response_free(&resp) }

        guard status == 0 else {
            dbg.error("PhpEngine", "php_execute failed with status \(status)")
            return nil
        }

        // ALWAYS forward PHP echo/print/var_dump output
        if let debugPtr = resp.debug, resp.debug_len > 0 {
            let debugStr = String(bytes: Data(bytes: debugPtr, count: resp.debug_len), encoding: .utf8) ?? ""
            if !debugStr.isEmpty {
                dbg.php(debugStr)
            }
        }

        // Prefer structured response (from phptoro_respond), fall back to body
        if let bodyPtr = resp.body, resp.body_len > 0 {
            let data = Data(bytes: bodyPtr, count: resp.body_len)
            if let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
                return json
            }
            let str = String(data: data, encoding: .utf8) ?? "(binary)"
            dbg.error("PhpEngine", "unparseable response: \(str.prefix(300))")
        }

        return nil
    }

    func shutdown() {
        guard initialized else { return }
        dbg.log("PhpEngine", "shutdown()")
        phptoro_php_shutdown()
        initialized = false
    }

    deinit {
        shutdown()
    }
}
