package tui

func tutorTUIQuickCaptureGrammarTopic() TutorTopic {
	return TutorTopic{
		Title:    "Quick Capture Grammar",
		Category: "TUI",
		Content: `# Opening Quick Capture

Press ;; from any view to open the Quick Capture overlay.
Type a task title followed by any sigils. Press ? inside
Quick Capture to toggle the inline syntax reference.

# Sigils

  ^type       Task type: ^epic ^story ^task ^bug ^idea
              Alias: type:bug

  !N/!word    Priority (1-5): !1 !urgent !high !normal !low
              Alias: pri:2

  #tag        Add a tag (multiple allowed): #backend #urgent
              Alias: tag:backend

  @date       Due date: @tomorrow @friday @+3d
              Alias: due:fri  due:3d

  ~recur      Recurrence: ~daily ~weekly ~2w ~monthly
              Alias: recur:weekly

  >parent     Parent task (short ID or title prefix): >42 >Login
              Alias: parent:42

  /section    Section: /inbox /next /waiting /someday
              Alias: sec:next

  $status     Initial status: $start $blocked $done $review
              $standby $hold $maybe $canceled
              Alias: status:done

  %project    Target project (slug or name): %myproject
              Alias: p:myproject  (omitted in global capture)

# Global Search Equivalents

Global search ($status #tag @due ^type) uses the same sigil
prefixes for filtering, but targets existing tasks rather than
creating new ones. Press ? in Global Search to see the full
filter syntax reference.`,
	}
}
