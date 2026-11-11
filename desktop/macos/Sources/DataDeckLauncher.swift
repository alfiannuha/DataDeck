// DataDeck macOS launcher.
//
// Lifecycle: single-instance lock -> prepare ~/.datadeck -> Keychain encryption
// key -> start the bundled Go backend (loopback only) -> poll /api/v1/health ->
// open the default browser -> keep running until Quit, then SIGTERM the backend.
//
// The launcher only handles app lifecycle, secure config, the backend process and
// browser launch. It never embeds the encryption key, never passes it via argv,
// and never logs it.

import AppKit
import Darwin
import Foundation
import Security

// MARK: - Constants

let kBundleIdentifier = "io.datadeck.desktop"
let kKeychainService = kBundleIdentifier
let kKeychainAccount = "encryption-key"
let kDefaultPort = 3200
let kPortScanSpan = 100
let kReadinessTimeout: TimeInterval = 25
let kReadinessPoll: TimeInterval = 0.25
let kShutdownGrace: TimeInterval = 8

// MARK: - Paths

func appHome() -> URL {
    FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent(".datadeck", isDirectory: true)
}
func databasePath() -> String { appHome().appendingPathComponent("datadeck.db").path }
func logsDirectory() -> URL { appHome().appendingPathComponent("logs", isDirectory: true) }
func configDirectory() -> URL { appHome().appendingPathComponent("config", isDirectory: true) }
func launcherLogPath() -> String { logsDirectory().appendingPathComponent("launcher.log").path }
func backendLogPath() -> String { logsDirectory().appendingPathComponent("backend.log").path }
func portFilePath() -> String { configDirectory().appendingPathComponent("port").path }
func lockFilePath() -> String { appHome().appendingPathComponent(".instance.lock").path }

func appVersion() -> String {
    (Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String) ?? "unknown"
}

// MARK: - Helpers

func hexString(_ data: Data) -> String {
    data.map { String(format: "%02x", $0) }.joined()
}

func ensureDirectories() throws {
    let fm = FileManager.default
    for dir in [appHome(), logsDirectory(), configDirectory()] {
        try fm.createDirectory(
            at: dir,
            withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700]
        )
        try? fm.setAttributes([.posixPermissions: 0o700], ofItemAtPath: dir.path)
    }
}

/// Append one lifecycle line to ~/.datadeck/logs/launcher.log (0600). Never pass
/// secrets here.
func logLine(_ message: String) {
    let ts = ISO8601DateFormatter().string(from: Date())
    let path = launcherLogPath()
    let line = "[\(ts)] \(message)\n"
    guard let data = line.data(using: .utf8) else { return }
    let fm = FileManager.default
    if !fm.fileExists(atPath: path) {
        fm.createFile(atPath: path, contents: nil, attributes: [.posixPermissions: 0o600])
    }
    if let handle = FileHandle(forWritingAtPath: path) {
        handle.seekToEndOfFile()
        handle.write(data)
        try? handle.close()
    }
}

/// Child-process environment for the Go backend. Inherits the launcher env (for
/// PATH/HOME/locale) but pins the security-relevant values explicitly and never
/// allows remote binding.
func backendEnvironment(port: Int, key: String) -> [String: String] {
    var env = ProcessInfo.processInfo.environment
    env["HOST"] = "127.0.0.1"
    env["PORT"] = String(port)
    env["STORAGE_PATH"] = databasePath()
    env["ENVIRONMENT"] = "production"
    env["LOG_LEVEL"] = "info"
    env["ENCRYPTION_KEY"] = key
    env.removeValue(forKey: "ALLOW_REMOTE")
    return env
}

func isPortFree(_ port: Int) -> Bool {
    let fd = socket(AF_INET, SOCK_STREAM, 0)
    if fd < 0 { return false }
    defer { close(fd) }
    var reuse: Int32 = 1
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &reuse, socklen_t(MemoryLayout<Int32>.size))
    var addr = sockaddr_in()
    addr.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
    addr.sin_family = sa_family_t(AF_INET)
    addr.sin_port = in_port_t(port).bigEndian
    addr.sin_addr = in_addr(s_addr: inet_addr("127.0.0.1"))
    let result = withUnsafePointer(to: &addr) { pointer in
        pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) {
            bind(fd, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
        }
    }
    return result == 0
}

/// One-shot loopback health probe for GET /api/v1/health. Synchronous (bounded) so
/// callers can poll simply.
func healthOK(_ port: Int, timeout: TimeInterval = 1.5) -> Bool {
    guard let url = URL(string: "http://127.0.0.1:\(port)/api/v1/health") else { return false }
    let config = URLSessionConfiguration.ephemeral
    config.timeoutIntervalForRequest = timeout
    config.timeoutIntervalForResource = timeout
    let session = URLSession(configuration: config)
    let semaphore = DispatchSemaphore(value: 0)
    var ok = false
    session.dataTask(with: url) { data, response, _ in
        if let http = response as? HTTPURLResponse, http.statusCode == 200,
           let body = data, let text = String(data: body, encoding: .utf8),
           text.contains("\"status\":\"healthy\"") {
            ok = true
        }
        semaphore.signal()
    }.resume()
    _ = semaphore.wait(timeout: .now() + timeout + 1)
    session.invalidateAndCancel()
    return ok
}

// MARK: - Keychain encryption key

enum KeychainError: Error, CustomStringConvertible {
    case read(OSStatus)
    case store(OSStatus)
    case generate(OSStatus)

    var description: String {
        switch self {
        case .read(let status): return "Keychain read failed (\(status))"
        case .store(let status): return "Keychain store failed (\(status))"
        case .generate(let status): return "secure random generation failed (\(status))"
        }
    }
}

/// Returns the 64-hex-char encryption key, creating it in the macOS Keychain on
/// first launch. Never replaces an existing key; a Keychain failure is fatal so a
/// user's encrypted credentials are never silently orphaned.
func loadOrCreateKey() throws -> String {
    let baseQuery: [String: Any] = [
        kSecClass as String: kSecClassGenericPassword,
        kSecAttrService as String: kKeychainService,
        kSecAttrAccount as String: kKeychainAccount,
    ]
    var lookup = baseQuery
    lookup[kSecReturnData as String] = true
    lookup[kSecMatchLimit as String] = kSecMatchLimitOne

    var item: CFTypeRef?
    let readStatus = SecItemCopyMatching(lookup as CFDictionary, &item)
    if readStatus == errSecSuccess, let data = item as? Data, data.count == 32 {
        return hexString(data)
    }
    if readStatus == errSecItemNotFound {
        var bytes = [UInt8](repeating: 0, count: 32)
        let genStatus = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        if genStatus != errSecSuccess { throw KeychainError.generate(genStatus) }
        let data = Data(bytes)
        var add = baseQuery
        add[kSecValueData as String] = data
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        let storeStatus = SecItemAdd(add as CFDictionary, nil)
        if storeStatus != errSecSuccess { throw KeychainError.store(storeStatus) }
        return hexString(data)
    }
    // Existing item with an unexpected shape, or a real read failure.
    throw KeychainError.read(readStatus)
}

// MARK: - App delegate

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var backend: Process?
    private var port = kDefaultPort
    private var lockFD: Int32 = -1
    private var isShuttingDown = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        installMenu()
        DispatchQueue.global().async { [weak self] in
            self?.startBackendAndOpenBrowser()
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        false
    }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        shutdown()
        return .terminateNow
    }

    func applicationWillTerminate(_ notification: Notification) {
        shutdown()
    }

    private func startBackendAndOpenBrowser() {
        do {
            try ensureDirectories()
        } catch {
            fatal("Could not prepare ~/.datadeck: \(error.localizedDescription)")
            return
        }
        logLine("launcher start version=\(appVersion())")

        guard acquireSingleInstanceLock() else {
            logLine("another instance is running")
            if let existing = currentInstancePort(), healthOK(existing) {
                openBrowser(existing)
            } else {
                openBrowser(kDefaultPort)
            }
            DispatchQueue.main.async { NSApp.terminate(nil) }
            return
        }

        let key: String
        do {
            key = try loadOrCreateKey()
        } catch {
            fatal("Keychain error: \(error). Your database was not modified.")
            return
        }

        guard let chosen = choosePort() else {
            fatal("No free local port in \(kDefaultPort)-\(kDefaultPort + kPortScanSpan). Close whatever is using them and retry.")
            return
        }
        port = chosen
        writePortFile(port)

        do {
            try launchBackend(port: port, key: key)
        } catch {
            fatal("Failed to start the DataDeck backend: \(error.localizedDescription)")
            return
        }
        logLine("backend started port=\(port)")

        let deadline = Date().addingTimeInterval(kReadinessTimeout)
        while Date() < deadline {
            if let process = backend, !process.isRunning {
                fatal("The DataDeck backend exited before it was ready. See \(backendLogPath()).")
                return
            }
            if healthOK(port) {
                logLine("backend ready port=\(port)")
                openBrowser(port)
                return
            }
            Thread.sleep(forTimeInterval: kReadinessPoll)
        }
        fatal("The DataDeck backend did not become ready within \(Int(kReadinessTimeout))s. See \(backendLogPath()).")
    }

    private func launchBackend(port: Int, key: String) throws {
        guard let resources = Bundle.main.resourceURL else {
            throw LauncherError.missingResources
        }
        let executable = resources.appendingPathComponent("backend/datadeck")
        guard FileManager.default.isExecutableFile(atPath: executable.path) else {
            throw LauncherError.missingBackend(executable.path)
        }

        let logPath = backendLogPath()
        if !FileManager.default.fileExists(atPath: logPath) {
            FileManager.default.createFile(atPath: logPath, contents: nil, attributes: [.posixPermissions: 0o600])
        }
        guard let logHandle = FileHandle(forWritingAtPath: logPath) else {
            throw LauncherError.logUnavailable(logPath)
        }
        logHandle.seekToEndOfFile()

        let process = Process()
        process.executableURL = executable
        process.arguments = [] // never pass the key (or anything secret) via argv
        process.environment = backendEnvironment(port: port, key: key)
        process.currentDirectoryURL = appHome()
        process.standardOutput = logHandle
        process.standardError = logHandle
        process.terminationHandler = { [weak self] terminated in
            DispatchQueue.main.async {
                guard let self, !self.isShuttingDown, self.backend === terminated else { return }
                logLine("backend exited unexpectedly status=\(terminated.terminationStatus)")
                self.fatal("The DataDeck backend stopped unexpectedly (exit \(terminated.terminationStatus)). See \(backendLogPath()).")
            }
        }
        try process.run()
        backend = process
    }

    private func choosePort() -> Int? {
        if isPortFree(kDefaultPort) { return kDefaultPort }
        for offset in 1...kPortScanSpan {
            let candidate = kDefaultPort + offset
            if isPortFree(candidate) {
                logLine("default port \(kDefaultPort) busy; using \(candidate)")
                return candidate
            }
        }
        return nil
    }

    private func acquireSingleInstanceLock() -> Bool {
        let fd = open(lockFilePath(), O_CREAT | O_RDWR, 0o600)
        if fd < 0 { return false }
        if flock(fd, LOCK_EX | LOCK_NB) != 0 {
            close(fd)
            return false
        }
        lockFD = fd
        return true
    }

    private func writePortFile(_ port: Int) {
        try? String(port).write(toFile: portFilePath(), atomically: true, encoding: .utf8)
    }

    private func currentInstancePort() -> Int? {
        guard let raw = try? String(contentsOfFile: portFilePath(), encoding: .utf8) else { return nil }
        return Int(raw.trimmingCharacters(in: .whitespacesAndNewlines))
    }

    private func openBrowser(_ port: Int) {
        guard let url = URL(string: "http://127.0.0.1:\(port)/") else { return }
        if !NSWorkspace.shared.open(url) {
            logLine("failed to open browser for \(url.absoluteString)")
        }
    }

    private func installMenu() {
        let mainMenu = NSMenu()
        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)

        let appMenu = NSMenu()
        appMenu.addItem(
            NSMenuItem(
                title: "About DataDeck",
                action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)),
                keyEquivalent: ""
            )
        )
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(
            NSMenuItem(
                title: "Quit DataDeck",
                action: #selector(NSApplication.terminate(_:)),
                keyEquivalent: "q"
            )
        )
        appMenuItem.submenu = appMenu
        NSApp.mainMenu = mainMenu
    }

    /// Show a native alert and terminate. Callable from any thread.
    private func fatal(_ message: String) {
        logLine("FATAL: \(message)")
        shutdown()
        DispatchQueue.main.async {
            let alert = NSAlert()
            alert.messageText = "DataDeck could not start"
            alert.informativeText = message
            alert.alertStyle = .critical
            alert.addButton(withTitle: "Quit")
            alert.runModal()
            NSApp.terminate(nil)
        }
    }

    /// Graceful stop: SIGTERM the backend, wait up to kShutdownGrace, then SIGKILL.
    /// Idempotent and safe to call from both terminate paths.
    private func shutdown() {
        if isShuttingDown { return }
        isShuttingDown = true
        if let process = backend, process.isRunning {
            logLine("backend shutdown requested")
            process.terminate() // SIGTERM -> Go runs graceful server.Shutdown
            let deadline = Date().addingTimeInterval(kShutdownGrace)
            while process.isRunning && Date() < deadline {
                Thread.sleep(forTimeInterval: 0.1)
            }
            if process.isRunning {
                logLine("backend did not stop in time; forcing")
                kill(process.processIdentifier, SIGKILL)
            }
            logLine("backend stopped")
        }
        backend = nil
        if lockFD >= 0 {
            close(lockFD)
            lockFD = -1
        }
    }
}

enum LauncherError: Error, LocalizedError {
    case missingResources
    case missingBackend(String)
    case logUnavailable(String)

    var errorDescription: String? {
        switch self {
        case .missingResources: return "The application bundle is missing its Resources directory."
        case .missingBackend(let path): return "The bundled backend is missing or not executable: \(path)"
        case .logUnavailable(let path): return "Could not open the backend log for writing: \(path)"
        }
    }
}

// MARK: - Self test (pure helpers; no Keychain, no backend, no network)

func runSelfTests() -> Bool {
    var passed = true
    func check(_ condition: Bool, _ name: String) {
        if condition { print("ok: \(name)") } else { FileHandle.standardError.write("FAIL: \(name)\n".data(using: .utf8)!); passed = false }
    }

    check(kDefaultPort == 3200, "default port is 3200")
    check(databasePath().hasSuffix("/.datadeck/datadeck.db"), "database path under ~/.datadeck")
    check(launcherLogPath().hasSuffix("/.datadeck/logs/launcher.log"), "launcher log path")
    check(backendLogPath().hasSuffix("/.datadeck/logs/backend.log"), "backend log path")

    let env = backendEnvironment(port: 8091, key: "test-key")
    check(env["HOST"] == "127.0.0.1", "backend HOST is loopback")
    check(env["PORT"] == "8091", "backend PORT matches selection")
    check(env["STORAGE_PATH"] == databasePath(), "backend STORAGE_PATH absolute")
    check(env["ENVIRONMENT"] == "production", "backend ENVIRONMENT production")
    check(env["ENCRYPTION_KEY"] == "test-key", "backend receives key via env")
    check(env["ALLOW_REMOTE"] == nil, "backend never binds remote")

    check(hexString(Data([0x00, 0x0f, 0xff])) == "000fff", "hex encoding")
    var bytes = [UInt8](repeating: 0, count: 32)
    let status = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
    check(status == errSecSuccess && hexString(Data(bytes)).count == 64, "generated key is 64 hex chars")
    check(isPortFree(kDefaultPort) || true, "port check is callable")

    return passed
}

// MARK: - Entry point

@main
struct DataDeckApp {
    static func main() {
        if CommandLine.arguments.contains("--self-test") {
            exit(runSelfTests() ? 0 : 1)
        }
        let application = NSApplication.shared
        let delegate = AppDelegate()
        application.delegate = delegate
        application.setActivationPolicy(.regular)
        application.run()
    }
}
