import CryptoKit
import SwiftUI
import DocketReminders

extension ReminderModel {
    /// One line for the Sync sheet's Reminders row.
    var summary: String {
        guard settings.enabled else { return "Off" }
        switch report?.outcome {
        case .permissionDenied: return "Not allowed"
        case .permissionNotDetermined: return "Needs permission"
        case .scheduled: return "\(report?.scheduledCount ?? 0) scheduled"
        default: return "On"
        }
    }

    /// What the reminders are doing, in a person's words.
    var statusText: String {
        guard settings.enabled else {
            return "Reminders are off. Turn them on to be notified when a check-in day arrives, a timed deadline is due, or a deferral ends."
        }
        guard let report else { return "Checking what to schedule…" }
        switch report.outcome {
        case .permissionDenied:
            return "Notifications are turned off for Docket in Settings, so nothing is scheduled. Your tasks are unaffected."
        case .permissionNotDetermined:
            return "Docket has not been allowed to notify yet. Turn reminders off and on again to be asked."
        case .noWorkspace:
            return "No workspace is open, so nothing is scheduled."
        case .planFailed:
            return "The reminders could not be planned: \(report.error ?? "unknown error"). Scheduled ones were kept."
        case .disabled:
            return "Reminders are off."
        case .scheduled:
            let count = report.scheduledCount
            var text = count == 1 ? "1 reminder scheduled." : "\(count) reminders scheduled."
            if let next = report.next {
                text += " Next: \(next.title), \(next.fireAt.formatted(date: .abbreviated, time: .shortened))."
            }
            if report.truncated {
                text += " Only the soonest are scheduled; more follow as these pass."
            }
            return text
        }
    }
}

/// Opt-in reminders: the switch, the check-in time of day, what is scheduled,
/// what a permission denial means, how times follow the time zone, and the
/// actions recently taken from notifications.
struct ReminderSettingsView: View {
    let reminders: ReminderModel
    @Environment(\.openURL) private var openURL

    var body: some View {
        List {
            Section {
                Toggle("Remind Me", isOn: Binding(
                    get: { reminders.settings.enabled },
                    set: { enabled in
                        var settings = reminders.settings
                        settings.enabled = enabled
                        Task { await reminders.update(settings) }
                    }
                ))
                .accessibilityIdentifier("reminders.enabled")
                DatePicker("Check-in time", selection: Binding(
                    get: { checkInTime },
                    set: { time in
                        let parts = Calendar.current.dateComponents([.hour, .minute], from: time)
                        var settings = reminders.settings
                        settings.checkInHour = parts.hour ?? 9
                        settings.checkInMinute = parts.minute ?? 0
                        Task { await reminders.update(settings) }
                    }
                ), displayedComponents: .hourAndMinute)
                .disabled(!reminders.settings.enabled)
                .accessibilityIdentifier("reminders.checkInTime")
            } footer: {
                Text("A check-in day is announced at this time. Timed deadlines are announced when due, and a deferral when it ends.")
            }
            Section {
                Text(reminders.statusText)
                    .accessibilityIdentifier("reminders.status")
                if reminders.report?.outcome == .permissionDenied {
                    Button("Open Settings") {
                        if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
                    }
                    .accessibilityIdentifier("reminders.openSettings")
                }
                LabeledContent("Time zone", value: reminders.timeZone)
                    .accessibilityIdentifier("reminders.timeZone")
            } header: {
                Text("Status")
            } footer: {
                Text("Check-ins and timed deadlines keep their local time when this iPhone changes time zone; a deferral ends at the same moment everywhere.")
            }
            if !reminders.activity.isEmpty {
                Section("Recent") {
                    ForEach(Array(reminders.activity.enumerated()), id: \.element.id) { index, entry in
                        VStack(alignment: .leading) {
                            Text(entry.summary)
                            Text(entry.at.formatted(date: .omitted, time: .shortened))
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                        .accessibilityElement(children: .combine)
                        .accessibilityIdentifier("reminders.activity.\(index)")
                    }
                }
            }
            if LaunchConfiguration.current.isUITest {
                Section("Build") {
                    Text(Self.buildDigests)
                        .font(.caption.monospaced())
                        .accessibilityIdentifier("reminders.build")
                }
            }
        }
        .navigationTitle("Reminders")
        .task { await reminders.refresh() }
    }

    private var checkInTime: Date {
        Calendar.current.date(
            bySettingHour: reminders.settings.checkInHour, minute: reminders.settings.checkInMinute, second: 0, of: Date()
        ) ?? Date()
    }

    /// The running app's executables by name and digest, which a device test
    /// compares with the build it installed.
    static let buildDigests: String = {
        let bundle = Bundle.main.bundleURL
        return ["Docket", "Docket.debug.dylib"].compactMap { name -> String? in
            guard let data = try? Data(contentsOf: bundle.appendingPathComponent(name)) else { return nil }
            let digest = SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
            return "\(name)=sha256:\(digest)"
        }.joined(separator: ";")
    }()
}
