import Foundation

// MARK: Conflict diffs

/// What one text changed in another, line by line, in the shape of a unified
/// diff: hunks of changed lines with a little unchanged context around them.
/// A conflict's versions are read against the version every side started
/// from, so the lines a device changed stand apart from the file they sit in.
public struct ConflictDiff: Hashable, Sendable {
    public enum Kind: Hashable, Sendable {
        case context
        case added
        case removed
    }

    public struct Line: Hashable, Sendable {
        public var kind: Kind
        public var text: String
        /// The file's last line where it does not end in a newline, which is
        /// itself a difference when the other side's does.
        public var missingNewline: Bool
    }

    public struct Hunk: Hashable, Sendable {
        /// One-based, and the line before the hunk where the count is zero,
        /// as `git diff` writes them.
        public var oldStart: Int
        public var oldCount: Int
        public var newStart: Int
        public var newCount: Int
        public var lines: [Line]

        public var header: String {
            "@@ -\(Self.range(oldStart, oldCount)) +\(Self.range(newStart, newCount)) @@"
        }

        private static func range(_ start: Int, _ count: Int) -> String {
            count == 1 ? "\(start)" : "\(start),\(count)"
        }
    }

    public var hunks: [Hunk]
    public var added: Int
    public var removed: Int

    public var isEmpty: Bool { hunks.isEmpty }

    /// The diff taking `old` to `new`; a nil side is a file that does not
    /// exist, so every line of the other is added or removed.
    public init(from old: String?, to new: String?, context: Int = 3) {
        let before = Self.entries(old)
        let after = Self.entries(new)
        var removals: Set<Int> = []
        var insertions: Set<Int> = []
        for change in after.difference(from: before) {
            switch change {
            case .remove(let offset, _, _): removals.insert(offset)
            case .insert(let offset, _, _): insertions.insert(offset)
            }
        }

        // Every line of both sides in reading order, a removal ahead of the
        // insertion that replaced it.
        var steps: [Step] = []
        var oldIndex = 0
        var newIndex = 0
        while oldIndex < before.count || newIndex < after.count {
            if oldIndex < before.count, removals.contains(oldIndex) {
                steps.append(Step(kind: .removed, entry: before[oldIndex], oldIndex: oldIndex, newIndex: newIndex))
                oldIndex += 1
            } else if newIndex < after.count, insertions.contains(newIndex) {
                steps.append(Step(kind: .added, entry: after[newIndex], oldIndex: oldIndex, newIndex: newIndex))
                newIndex += 1
            } else {
                steps.append(Step(kind: .context, entry: after[newIndex], oldIndex: oldIndex, newIndex: newIndex))
                oldIndex += 1
                newIndex += 1
            }
        }

        // Each change with its context, runs that touch or overlap joined.
        var ranges: [Range<Int>] = []
        for (index, step) in steps.enumerated() where step.kind != .context {
            let range = max(0, index - context)..<min(steps.count, index + context + 1)
            if let last = ranges.last, range.lowerBound <= last.upperBound {
                ranges[ranges.count - 1] = last.lowerBound..<range.upperBound
            } else {
                ranges.append(range)
            }
        }

        hunks = ranges.map { range in
            let slice = steps[range]
            let oldCount = slice.count { $0.kind != .added }
            let newCount = slice.count { $0.kind != .removed }
            let first = slice[range.lowerBound]
            return Hunk(
                oldStart: oldCount == 0 ? first.oldIndex : first.oldIndex + 1,
                oldCount: oldCount,
                newStart: newCount == 0 ? first.newIndex : first.newIndex + 1,
                newCount: newCount,
                lines: slice.map { Line(kind: $0.kind, text: $0.entry.text, missingNewline: !$0.entry.terminated) }
            )
        }
        added = insertions.count
        removed = removals.count
    }

    private struct Entry: Hashable {
        var text: String
        var terminated: Bool
    }

    private struct Step {
        var kind: Kind
        var entry: Entry
        var oldIndex: Int
        var newIndex: Int
    }

    private static func entries(_ text: String?) -> [Entry] {
        guard let text, !text.isEmpty else { return [] }
        var parts = text.split(omittingEmptySubsequences: false, whereSeparator: \.isNewline)
        let terminated = text.last?.isNewline == true
        if terminated { parts.removeLast() }
        return parts.enumerated().map { index, part in
            Entry(text: String(part), terminated: terminated || index < parts.count - 1)
        }
    }
}
