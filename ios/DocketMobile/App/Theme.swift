import SwiftUI
import DocketShared

extension View {
    /// Lays a list or form on Docket's canvas: the system's grouped
    /// background by day and Tokyo Night by night. Its rows take
    /// `DocketPalette.surface` from the `Group` each list wraps them in.
    func docketCanvas() -> some View {
        scrollContentBackground(.hidden)
            .background(DocketPalette.canvas.ignoresSafeArea())
    }
}

/// Lays its children left to right and starts a new line when the next one
/// would not fit, so a row of chips wraps instead of running off the edge.
struct WrapLayout: Layout {
    var spacing: CGFloat = 8
    var lineSpacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let lines = arrange(width: proposal.width ?? .infinity, subviews: subviews)
        let width = lines.map(\.width).max() ?? 0
        let height = lines.map(\.height).reduce(0, +) + lineSpacing * CGFloat(max(0, lines.count - 1))
        return CGSize(width: width, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var y = bounds.minY
        for line in arrange(width: bounds.width, subviews: subviews) {
            var x = bounds.minX
            for index in line.indices {
                let size = subviews[index].sizeThatFits(ProposedViewSize(width: bounds.width, height: nil))
                subviews[index].place(at: CGPoint(x: x, y: y + (line.height - size.height) / 2),
                                      proposal: ProposedViewSize(width: min(size.width, bounds.width), height: size.height))
                x += min(size.width, bounds.width) + spacing
            }
            y += line.height + lineSpacing
        }
    }

    private struct Line {
        var indices: [Int] = []
        var width: CGFloat = 0
        var height: CGFloat = 0
    }

    private func arrange(width: CGFloat, subviews: Subviews) -> [Line] {
        var lines: [Line] = []
        var current = Line()
        for index in subviews.indices {
            let size = subviews[index].sizeThatFits(ProposedViewSize(width: width, height: nil))
            let itemWidth = min(size.width, width)
            let needed = current.indices.isEmpty ? itemWidth : current.width + spacing + itemWidth
            if needed > width, !current.indices.isEmpty {
                lines.append(current)
                current = Line()
            }
            current.width = current.indices.isEmpty ? itemWidth : current.width + spacing + itemWidth
            current.height = max(current.height, size.height)
            current.indices.append(index)
        }
        if !current.indices.isEmpty { lines.append(current) }
        return lines
    }
}

// MARK: - Zoom transitions

private struct ZoomNamespaceKey: EnvironmentKey {
    static let defaultValue: Namespace.ID? = nil
}

extension EnvironmentValues {
    /// The shell's namespace for zoom transitions: a row or chip registers as
    /// a source in it, and the screen it opens zooms out of that source.
    var zoomNamespace: Namespace.ID? {
        get { self[ZoomNamespaceKey.self] }
        set { self[ZoomNamespaceKey.self] = newValue }
    }
}

private struct ZoomSource<ID: Hashable>: ViewModifier {
    let id: ID
    @Environment(\.zoomNamespace) private var namespace

    func body(content: Content) -> some View {
        if let namespace {
            content.matchedTransitionSource(id: id, in: namespace)
        } else {
            content
        }
    }
}

private struct ZoomDestination<ID: Hashable>: ViewModifier {
    let id: ID
    @Environment(\.zoomNamespace) private var namespace

    func body(content: Content) -> some View {
        if let namespace {
            // The zoom animates in and out, but no drag dismisses, so a
            // triage card still swipes and a scrolled ritual still scrolls.
            // Only presentations zoom: a pushed screen would take its
            // drag-to-dismiss from pull-to-refresh, which this cannot stop.
            content
                .navigationTransition(.zoom(sourceID: id, in: namespace))
                .interactiveDismissDisabled()
        } else {
            content
        }
    }
}

extension View {
    /// Marks this view as where a zoom transition with `id` begins.
    func zoomSource(_ id: some Hashable) -> some View {
        modifier(ZoomSource(id: id))
    }

    /// Opens this screen out of the zoom source with `id`.
    func zoomDestination(_ id: some Hashable) -> some View {
        modifier(ZoomDestination(id: id))
    }
}
