import Foundation
import XCTest
import TTEngine
@testable import tt

final class SchedulingTests: XCTestCase {
    func testCheckInDraftPreservesScheduleAndRejectsInvalidIntervals() async throws {
        let engine = try await openEngine()
        let task = try await engine.capture(title: "Weekly review").task
        _ = try await engine.checkIn([task.locator], .set(date: "2030-06-10", repeatDays: 7))
        let stored = try await engine.task(task.locator).task
        XCTAssertEqual(stored.checkInRepeatDays, 7)
        var draft = CheckInDraft(task: stored)
        XCTAssertEqual(draft.repeatDays, "7")
        XCTAssertTrue(draft.isUnchanged(from: stored))
        _ = try await engine.checkIn([task.locator], XCTUnwrap(draft.action))
        let reread = try await engine.task(task.locator).task
        XCTAssertEqual(reread.nextCheckIn, stored.nextCheckIn)
        XCTAssertEqual(reread.checkInRepeatDays, 7)
        XCTAssertEqual(reread.updatedAt, stored.updatedAt)
        for invalid in ["0", "-1", "7 days", "1.5", "3651", "999999999999999999999999", "７"] {
            draft.repeatDays = invalid
            XCTAssertNil(draft.action, invalid)
        }
        draft.repeatDays = " "
        _ = try await engine.checkIn([task.locator], XCTUnwrap(draft.action))
        let once = try await engine.task(task.locator).task
        XCTAssertEqual(once.checkInRepeatDays ?? 0, 0)
        XCTAssertEqual(once.nextCheckIn, stored.nextCheckIn)
    }

    private func openEngine() async throws -> TTEngine {
        let engine = TTEngine()
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try await engine.open(dataDirectory: root.appendingPathComponent("workspace"),
                              runtimeDirectory: root.appendingPathComponent("runtime"),
                              deviceID: "schedule-tests", createIfMissing: true)
        addTeardownBlock {
            try await engine.close()
            try FileManager.default.removeItem(at: root)
        }
        return engine
    }

    func testStructuredScheduleRoundTripsThroughTheRealEngine() async throws {
        let engine = try await openEngine()
        var fields = TaskFields()
        fields.title = "A timed task"
        fields.hasDue = true
        fields.includesDueTime = true
        fields.due = date(10, 15, 30)
        fields.hasDefer = true
        fields.deferUntil = date(9, 9, 45)
        let captured = try await engine.capture(fields.capture(projectID: nil, destination: .inbox))
        XCTAssertEqual(captured.task.dueDate, fields.due)
        XCTAssertEqual(captured.task.deferredUntil, fields.deferUntil)
        let original = TaskFields(captured.task)
        XCTAssertTrue(original.includesDueTime)
        var edited = original
        edited.due = date(11, 15, 30)
        edited.deferUntil = date(9, 10, 15)
        let saved = try await engine.edit(captured.task.locator, edited.edit(from: original))
        XCTAssertEqual(saved.task.dueDate, edited.due)
        XCTAssertEqual(saved.task.deferredUntil, edited.deferUntil)
        let reread = try await engine.task(captured.task.locator)
        XCTAssertEqual(reread.task.dueDate, edited.due)
        XCTAssertEqual(reread.task.deferredUntil, edited.deferUntil)
        let noChange = try await engine.edit(captured.task.locator, edited.edit(from: edited))
        XCTAssertFalse(noChange.changed)
    }

    func testAttentionUsesOneSignalAndKeepsTerminalWorkQuiet() async throws {
        let engine = try await openEngine()
        var task = try await engine.capture(title: "Attention").task
        let now = date(10, 12)
        task.priority = 1
        task.review = true
        task.dueDate = date(10, 9)
        XCTAssertTrue(TaskAttention(task: task, now: now).overdue)
        task.dueDate = date(10, 15)
        XCTAssertEqual(TaskAttention(task: task, now: now).label, "Review needed")
        task.review = false
        XCTAssertTrue(TaskAttention(task: task, now: now).showsDeadline)
        task.dueDate = nil
        task.nextCheckIn = date(10)
        XCTAssertEqual(TaskAttention(task: task, now: now).label, "Check-in due")
        task.deferredUntil = date(11)
        XCTAssertEqual(TaskAttention(task: task, now: now).label, "High priority")
        task.state = .done
        task.dueDate = date(9)
        XCTAssertNil(TaskAttention(task: task, now: now).label)
    }

    private func date(_ day: Int, _ hour: Int = 0, _ minute: Int = 0) -> Date {
        TaskSchedule.calendar.date(from: DateComponents(year: 2030, month: 6, day: day, hour: hour, minute: minute))!
    }

    func testReschedulingKeepsDeadlineClockAndDetectsTimeOnlyChanges() throws {
        var original = TaskFields()
        original.hasDue = true
        original.includesDueTime = true
        original.due = date(10, 15, 30)
        var edited = original
        edited.due = TaskSchedule.calendar.date(byAdding: .day, value: 1, to: original.due)!
        XCTAssertEqual(edited.edit(from: original).due, "2030-06-11 15:30")
        edited.due = date(10, 16, 45)
        XCTAssertEqual(edited.edit(from: original).due, "2030-06-10 16:45")
        XCTAssertNotEqual(original, edited)
        edited = original
        edited.title = "A title-only edit"
        XCTAssertNil(edited.edit(from: original).due)
        edited.includesDueTime = false
        XCTAssertEqual(edited.edit(from: original).due, "2030-06-10")
        edited.hasDue = false
        XCTAssertEqual(edited.edit(from: original).due, "")
    }

    func testDeferralKeepsItsInstantIncludingTimeOnlyEditsAndRelaunch() throws {
        var original = TaskFields()
        original.hasDefer = true
        original.deferUntil = date(10, 15, 30)
        var edited = original
        edited.deferUntil = date(10, 16, 45)
        let encoded = try JSONEncoder().encode(EditDraft(original: original, fields: edited))
        let restored = try JSONDecoder().decode(EditDraft.self, from: encoded)
        XCTAssertNotEqual(restored.original, restored.fields)
        let wire = try XCTUnwrap(restored.fields.edit(from: restored.original).deferUntil)
        XCTAssertEqual(try Date(wire, strategy: .iso8601), edited.deferUntil)
        XCTAssertEqual(edited.capture(projectID: nil, destination: .inbox).deferUntil, wire)
        edited = original
        edited.title = "Unrelated edit"
        XCTAssertNil(edited.edit(from: original).deferUntil)
    }

    func testLegacyDraftsRemainDateOnlyAndNewTimedDraftsRoundTrip() throws {
        var fields = TaskFields()
        fields.hasDue = true
        fields.due = date(10, 15, 30)
        // Older drafts have no time toggle, even though Date stored a clock.
        let data = try JSONEncoder().encode(fields)
        let restored = try JSONDecoder().decode(TaskFields.self, from: data)
        XCTAssertEqual(restored.dueValue, "2030-06-10")
        fields.includesDueTime = true
        let timed = try JSONDecoder().decode(TaskFields.self, from: JSONEncoder().encode(fields))
        XCTAssertEqual(timed.dueValue, "2030-06-10 15:30")
        fields.hasDefer = true
        fields.deferUntil = date(9, 17, 25)
        var legacy = try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(fields)) as? [String: Any])
        legacy.removeValue(forKey: "deferTimeEnabled")
        let old = try JSONDecoder().decode(TaskFields.self, from: JSONSerialization.data(withJSONObject: legacy))
        XCTAssertEqual(old.scheduledDeferral, date(9))
        var rescheduled = old
        rescheduled.scheduledDeferral = date(9, 14)
        XCTAssertNotEqual(old, rescheduled)
    }

    func testDateOnlyDeadlineLastsTheWholeDayButTimedDeadlineBecomesOverdue() {
        XCTAssertFalse(TaskSchedule.overdue(date(10), now: date(10, 23, 59)))
        XCTAssertTrue(TaskSchedule.overdue(date(10), now: date(11)))
        XCTAssertFalse(TaskSchedule.overdue(date(10, 15, 30), now: date(10, 15, 29)))
        XCTAssertTrue(TaskSchedule.overdue(date(10, 15, 30), now: date(10, 15, 31)))
        XCTAssertEqual(TaskSchedule.deadline(date(10)), date(10).formatted(date: .abbreviated, time: .omitted))
        XCTAssertEqual(TaskSchedule.deadline(date(10, 15, 30)), date(10, 15, 30).formatted(date: .abbreviated, time: .shortened))
    }
}
