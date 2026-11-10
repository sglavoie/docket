import ActivityKit
import AppIntents
import SwiftUI
import DocketShared
import WidgetKit

// MARK: Today's progress

/// Today's plan as a ring, for the Lock Screen and a small Home Screen tile:
/// how much is done, how much is left, and the next thing to do.
struct TodayProgressWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "docket.progress", provider: SnapshotProvider(view: .today)) { entry in
            TodayProgressContent(state: entry.state)
                .containerBackground(for: .widget) {
                    LinearGradient(colors: [DocketPalette.navy, DocketPalette.navyDeep], startPoint: .top, endPoint: .bottom)
                }
                .widgetURL(DeepLink.view(.today).url)
        }
        .configurationDisplayName("Today’s Progress")
        .description("How much of today’s plan is done.")
        .supportedFamilies([.accessoryCircular, .accessoryRectangular, .accessoryInline, .systemSmall])
    }
}

struct TodayProgressContent: View {
    let state: WidgetState
    @Environment(\.widgetFamily) private var family

    private var current: Bool { state.freshness == .current || state.freshness == .stale }
    private var done: Int { current ? state.snapshot?.doneToday ?? 0 : 0 }
    private var open: Int { current ? state.count(.today) ?? 0 : 0 }
    private var progress: Double { current ? state.snapshot?.todayProgress ?? 0 : 0 }
    private var next: WidgetTask? { state.snapshot?.doing ?? state.rows(.today).first }

    var body: some View {
        switch family {
        case .accessoryCircular:
            Gauge(value: progress) {
                Image(systemName: "sun.max.fill")
            } currentValueLabel: {
                Text(open == 0 && done > 0 ? "✓" : "\(open)")
            }
            .gaugeStyle(.accessoryCircularCapacity)
            .widgetAccentable()
        case .accessoryRectangular:
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 4) {
                    Image(systemName: "sun.max.fill")
                    Text(open == 0 && done > 0 ? "Today’s plan is done" : "Today · \(done) of \(done + open)")
                        .font(.headline)
                }
                .widgetAccentable()
                if let next, open > 0 {
                    Text(next.title).lineLimit(2).font(.caption)
                } else if !current {
                    Text("Open Docket to update").font(.caption)
                }
                if open > 0 {
                    ProgressView(value: progress).tint(.primary)
                }
            }
        case .accessoryInline:
            Text(open == 0 && done > 0 ? "Today’s plan is done" : "\(done) of \(done + open) done today")
        default:
            VStack(alignment: .leading, spacing: 8) {
                HStack {
                    Text("Today").font(.headline).foregroundStyle(.white)
                    Spacer()
                    Image(systemName: "sun.max.fill").foregroundStyle(DocketPalette.today)
                }
                Spacer(minLength: 0)
                ZStack {
                    Circle().stroke(.white.opacity(0.12), lineWidth: 8)
                    Circle()
                        .trim(from: 0, to: open == 0 && done > 0 ? 1 : progress)
                        .stroke(DocketPalette.brand, style: StrokeStyle(lineWidth: 8, lineCap: .round))
                        .rotationEffect(.degrees(-90))
                    VStack(spacing: 0) {
                        if open == 0 && done > 0 {
                            Image(systemName: "checkmark").font(.title2.weight(.bold)).foregroundStyle(DocketPalette.brand)
                        } else {
                            Text("\(done)").font(.system(.title2, design: .rounded).weight(.bold)).foregroundStyle(.white)
                            Text("of \(done + open)").font(.caption2).foregroundStyle(.white.opacity(0.6))
                        }
                    }
                }
                .frame(width: 76, height: 76)
                .frame(maxWidth: .infinity)
                Spacer(minLength: 0)
                Text(current ? (open == 0 ? (done > 0 ? "All done. Nice." : "Nothing planned") : "\(open) to go") : "Open Docket to update")
                    .font(.caption.weight(.medium))
                    .foregroundStyle(.white.opacity(0.7))
            }
        }
    }
}

// MARK: Now

/// The task in progress on the Home or Lock Screen, with a Done button.
struct NowWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "docket.now", provider: SnapshotProvider(view: .today)) { entry in
            NowWidgetContent(state: entry.state)
                .containerBackground(for: .widget) {
                    LinearGradient(colors: [DocketPalette.navy, DocketPalette.navyDeep], startPoint: .top, endPoint: .bottom)
                }
                .widgetURL(DeepLink.ritual(.now).url)
        }
        .configurationDisplayName("Now")
        .description("The task in progress, and a button to finish it.")
        .supportedFamilies([.systemSmall, .accessoryRectangular])
    }
}

struct NowWidgetContent: View {
    let state: WidgetState
    @Environment(\.widgetFamily) private var family

    private var doing: WidgetTask? {
        state.freshness == .expired || state.freshness == .missing ? nil : state.snapshot?.doing
    }

    var body: some View {
        if family == .accessoryRectangular {
            VStack(alignment: .leading, spacing: 2) {
                Label("Now", systemImage: "scope").font(.headline).widgetAccentable()
                Text(doing?.title ?? "Nothing in progress").font(.caption).lineLimit(2)
                if let since = state.snapshot?.doingSince, doing != nil {
                    Text(since, style: .relative).font(.caption2)
                }
            }
        } else {
            VStack(alignment: .leading, spacing: 6) {
                Label("Now", systemImage: "scope")
                    .font(.caption.weight(.bold))
                    .foregroundStyle(DocketPalette.state("doing"))
                if let doing {
                    Text(doing.title)
                        .font(.system(.headline, design: .rounded))
                        .foregroundStyle(.white)
                        .lineLimit(3)
                    if let since = state.snapshot?.doingSince {
                        Text(since, style: .timer)
                            .font(.caption.monospacedDigit())
                            .foregroundStyle(.white.opacity(0.6))
                    }
                    Spacer(minLength: 0)
                    Button(intent: CompleteTaskIntent(workspace: state.snapshot?.workspace ?? "", projectID: doing.projectID, taskID: doing.taskID)) {
                        Label("Done", systemImage: "checkmark")
                            .font(.caption.weight(.bold))
                            .frame(maxWidth: .infinity)
                    }
                    .tint(DocketPalette.brand)
                    .buttonStyle(.borderedProminent)
                } else {
                    Spacer(minLength: 0)
                    Text("Nothing in progress")
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(.white)
                    Text("Start a task and it shows here.")
                        .font(.caption)
                        .foregroundStyle(.white.opacity(0.6))
                }
            }
        }
    }
}

// MARK: Controls

/// Control Center, Lock Screen, and Action button entries: each opens a link.
struct QuickCaptureControl: ControlWidget {
    var body: some ControlWidgetConfiguration {
        StaticControlConfiguration(kind: "docket.control.capture") {
            ControlWidgetButton(action: OpenDocketLinkIntent(.capture)) {
                Label("Capture", systemImage: "plus.circle")
            }
        }
        .displayName("Capture to Docket")
        .description("Opens a new capture.")
    }
}

struct StartDayControl: ControlWidget {
    var body: some ControlWidgetConfiguration {
        StaticControlConfiguration(kind: "docket.control.startDay") {
            ControlWidgetButton(action: OpenDocketLinkIntent(.ritual(.startDay))) {
                Label("Start the Day", systemImage: "sunrise")
            }
        }
        .displayName("Start the Day")
        .description("Sorts your Inbox, one card at a time.")
    }
}

struct NowControl: ControlWidget {
    var body: some ControlWidgetConfiguration {
        StaticControlConfiguration(kind: "docket.control.now") {
            ControlWidgetButton(action: OpenDocketLinkIntent(.ritual(.now))) {
                Label("Now", systemImage: "scope")
            }
        }
        .displayName("Docket Now")
        .description("Opens the task in progress.")
    }
}

// MARK: Live Activity

/// The task in progress on the Lock Screen and in the Dynamic Island.
struct DocketLiveActivity: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: DocketActivityAttributes.self) { context in
            LiveActivityLockScreen(context: context)
                .activityBackgroundTint(DocketPalette.navyDeep)
                .activitySystemActionForegroundColor(DocketPalette.brand)
                .widgetURL(context.attributes.link.url)
        } dynamicIsland: { context in
            DynamicIsland {
                DynamicIslandExpandedRegion(.leading) {
                    Image(systemName: "scope")
                        .font(.title2)
                        .foregroundStyle(DocketPalette.state("doing"))
                        .padding(.leading, 4)
                }
                DynamicIslandExpandedRegion(.trailing) {
                    ActivityClock(state: context.state)
                        .font(.title3.monospacedDigit().weight(.semibold))
                        .foregroundStyle(DocketPalette.state("doing"))
                        .frame(maxWidth: 90)
                }
                DynamicIslandExpandedRegion(.center) {
                    Text(context.state.title).font(.headline).lineLimit(1)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    HStack {
                        Text(context.state.resumeNote ?? context.attributes.projectName)
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .lineLimit(2)
                        Spacer()
                        DoneButton(attributes: context.attributes)
                    }
                }
            } compactLeading: {
                Image(systemName: "scope").foregroundStyle(DocketPalette.state("doing"))
            } compactTrailing: {
                ActivityClock(state: context.state)
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(DocketPalette.state("doing"))
                    .frame(maxWidth: 48)
            } minimal: {
                Image(systemName: "scope").foregroundStyle(DocketPalette.state("doing"))
            }
            .widgetURL(context.attributes.link.url)
            .keylineTint(DocketPalette.state("doing"))
        }
    }
}

struct LiveActivityLockScreen: View {
    let context: ActivityViewContext<DocketActivityAttributes>

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Label(context.attributes.projectName.isEmpty ? "In progress" : context.attributes.projectName, systemImage: "scope")
                        .font(.caption.weight(.bold))
                        .foregroundStyle(DocketPalette.state("doing"))
                    Text(context.state.title)
                        .font(.system(.headline, design: .rounded))
                        .foregroundStyle(.white)
                        .lineLimit(2)
                }
                Spacer()
                ActivityClock(state: context.state)
                    .font(.system(.title2, design: .rounded).monospacedDigit().weight(.bold))
                    .foregroundStyle(.white)
                    .multilineTextAlignment(.trailing)
                    .frame(maxWidth: 110, alignment: .trailing)
            }
            if let focus = context.state.focus(at: .now) {
                ProgressView(timerInterval: focus, countsDown: true) { EmptyView() } currentValueLabel: { EmptyView() }
                    .tint(DocketPalette.state("doing"))
            }
            HStack {
                if let note = context.state.resumeNote, !note.isEmpty {
                    Label(note, systemImage: "bookmark.fill")
                        .font(.caption)
                        .foregroundStyle(.white.opacity(0.7))
                        .lineLimit(1)
                }
                Spacer()
                DoneButton(attributes: context.attributes)
            }
        }
        .padding(16)
    }
}

/// The focus countdown while one runs, else the time since work started.
struct ActivityClock: View {
    let state: DocketActivityAttributes.ContentState

    var body: some View {
        if let focus = state.focus(at: .now) {
            Text(timerInterval: focus, countsDown: true)
        } else {
            Text(state.doingSince, style: .timer)
        }
    }
}

struct DoneButton: View {
    let attributes: DocketActivityAttributes

    var body: some View {
        Button(intent: CompleteTaskIntent(workspace: attributes.workspace, projectID: attributes.projectID, taskID: attributes.taskID)) {
            Label("Done", systemImage: "checkmark")
                .font(.caption.weight(.bold))
        }
        .tint(DocketPalette.brand)
        .buttonStyle(.borderedProminent)
        .foregroundStyle(DocketPalette.navyDeep)
    }
}
