import AppIntents
import SwiftUI
import WidgetKit

/// Docket's controls, for Control Center, the menu bar, and the Lock Screen.
/// Each is one button that opens a `docket://` link: quick capture, quick find,
/// or Docket in the mode its counts point at. Like the widget, the agenda control
/// reads the pulse snapshot Docket.app keeps in the App Group container and
/// never the workspace.
enum DocketControlKind {
    static let capture = "docket.control.capture"
    static let find = "docket.control.find"
    static let agenda = DocketPulseSnapshot.agendaControlKind
}

@available(macOS 26.0, *)
struct QuickCaptureControl: ControlWidget {
    var body: some ControlWidgetConfiguration {
        StaticControlConfiguration(kind: DocketControlKind.capture) {
            ControlWidgetButton(action: OpenDocketLinkIntent(.capture(nil))) {
                Label("Quick Capture", systemImage: "plus.circle")
            }
        }
        .displayName("Quick Capture")
        .description("Opens Docket's quick capture over the app in front.")
    }
}

@available(macOS 26.0, *)
struct QuickFindControl: ControlWidget {
    var body: some ControlWidgetConfiguration {
        StaticControlConfiguration(kind: DocketControlKind.find) {
            ControlWidgetButton(action: OpenDocketLinkIntent(.find(nil))) {
                Label("Find Task", systemImage: "magnifyingglass")
            }
        }
        .displayName("Find Task")
        .description("Opens Docket's quick find, which searches every task.")
    }
}

@available(macOS 26.0, *)
struct AgendaControl: ControlWidget {
    var body: some ControlWidgetConfiguration {
        StaticControlConfiguration(kind: DocketControlKind.agenda, provider: AgendaControlProvider()) { value in
            ControlWidgetButton(action: OpenDocketLinkIntent(value.link)) {
                Label(value.title, systemImage: value.overdue > 0 ? "exclamationmark.circle" : "checklist")
            }
        }
        .displayName("Docket Agenda")
        .description("Shows what is overdue or due today, and opens Docket on it.")
    }
}

@available(macOS 26.0, *)
struct AgendaControlProvider: ControlValueProvider {
    var previewValue: AgendaControlValue { AgendaControlValue(overdue: 1, today: 3) }

    func currentValue() async throws -> AgendaControlValue {
        AgendaControlValue(snapshot: DocketPulseSnapshotStore(container: DocketSharedContainer.root())?.read())
    }
}
