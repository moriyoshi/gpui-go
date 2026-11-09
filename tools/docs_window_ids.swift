// Prints the on-screen window numbers owned by a process, one per line.
// examples/docshots passes them to screencapture.
import CoreGraphics

let pid = Int(CommandLine.arguments[1])!
let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
let windows = CGWindowListCopyWindowInfo(options, kCGNullWindowID) as! [[String: Any]]
for window in windows
where window[kCGWindowOwnerPID as String] as? Int == pid
    && window[kCGWindowLayer as String] as? Int == 0
{
    print(window[kCGWindowNumber as String] as! Int)
}
