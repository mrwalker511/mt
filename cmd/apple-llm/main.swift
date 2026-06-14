#if canImport(FoundationModels)
import FoundationModels
import Foundation

var lines: [String] = []
while let line = readLine(strippingNewline: false) {
    lines.append(line)
}
let prompt = lines.joined()

guard !prompt.isEmpty else {
    fputs("error: empty prompt\n", stderr)
    exit(1)
}

let sema = DispatchSemaphore(value: 0)
var exitCode: Int32 = 0

Task {
    do {
        let session = LanguageModelSession()
        let response = try await session.respond(to: prompt)
        print(response.content, terminator: "")
    } catch {
        fputs("error: \(error.localizedDescription)\n", stderr)
        exitCode = 1
    }
    sema.signal()
}

sema.wait()
exit(exitCode)
#else
import Foundation
fputs("error: FoundationModels is not available — requires macOS 26+ with Apple Intelligence enabled\n", stderr)
exit(1)
#endif
