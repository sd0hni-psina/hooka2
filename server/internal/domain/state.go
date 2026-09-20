package domain

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMaster Role = "master"
)

func (r Role) Valid() bool { return r == RoleOwner || r == RoleAdmin || r == RoleMaster }

// CanManage reports whether the role edits layout, plan and staff.
func (r Role) CanManage() bool { return r == RoleOwner || r == RoleAdmin }

type HookahState string

const (
	HookahActive   HookahState = "active"
	HookahExpired  HookahState = "expired"
	HookahFinished HookahState = "finished"
)

// Live hookahs occupy their table.
func (s HookahState) Live() bool { return s == HookahActive || s == HookahExpired }

type EventState string

const (
	EventPending   EventState = "pending"
	EventDue       EventState = "due"
	EventConfirmed EventState = "confirmed"
	EventMissed    EventState = "missed"
	EventCancelled EventState = "cancelled"
)

type FinishReason string

const (
	FinishGuestLeft FinishReason = "guest_left"
	FinishReplaced  FinishReason = "replaced"
	FinishMistake   FinishReason = "mistake"
	FinishShiftEnd  FinishReason = "shift_end"
)

func (r FinishReason) Valid() bool {
	switch r {
	case FinishGuestLeft, FinishReplaced, FinishMistake, FinishShiftEnd:
		return true
	}
	return false
}

type ServiceEvent struct {
	ID              string     `json:"id"`
	HookahID        string     `json:"hookah_id"`
	Seq             int        `json:"seq"`
	Kind            EventKind  `json:"kind"`
	Title           string     `json:"title"`
	DueAt           time.Time  `json:"due_at"`
	State           EventState `json:"state"`
	ClaimedBy       *string    `json:"claimed_by"`
	ClaimedByName   *string    `json:"claimed_by_name"`
	ClaimedAt       *time.Time `json:"claimed_at"`
	ConfirmedBy     *string    `json:"confirmed_by"`
	ConfirmedByName *string    `json:"confirmed_by_name"`
	ConfirmedAt     *time.Time `json:"confirmed_at"`
	ConfirmedEarly  bool       `json:"confirmed_early"`
}

type Hookah struct {
	ID            string         `json:"id"`
	TableID       string         `json:"table_id"`
	HallID        string         `json:"hall_id"`
	State         HookahState    `json:"state"`
	StartedAt     time.Time      `json:"started_at"`
	ExpiresAt     time.Time      `json:"expires_at"`
	FinishedAt    *time.Time     `json:"finished_at"`
	StartedBy     string         `json:"started_by"`
	StartedByName string         `json:"started_by_name"`
	Status        Status         `json:"status"`
	Events        []ServiceEvent `json:"events"`
}

// CheckMutable guards transfer and finish: a finished hookah accepts neither.
func CheckMutable(s HookahState) error {
	if s == HookahFinished {
		return ErrHookahFinished
	}
	return nil
}

type ConfirmDecision int

const (
	// ConfirmNow confirms an event that is due.
	ConfirmNow ConfirmDecision = iota + 1
	// ConfirmEarly confirms the nearest pending event of its kind.
	ConfirmEarly
	// ConfirmAlreadyDone means someone confirmed first; the caller returns the
	// current event with success, since confirming is idempotent for masters.
	ConfirmAlreadyDone
)

// DecideConfirm applies the confirmation rules of tech.md section 7 to one
// event given all events of its hookah.
func DecideConfirm(hookah HookahState, ev ServiceEvent, all []ServiceEvent) (ConfirmDecision, error) {
	if hookah == HookahFinished {
		return 0, ErrHookahFinished
	}
	switch ev.State {
	case EventConfirmed:
		return ConfirmAlreadyDone, nil
	case EventDue:
		return ConfirmNow, nil
	case EventPending:
		for _, o := range all {
			if o.ID != ev.ID && o.Kind == ev.Kind && o.State == EventPending && o.Seq < ev.Seq {
				return 0, ErrEventNotActionable
			}
		}
		return ConfirmEarly, nil
	}
	return 0, ErrEventNotActionable
}

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

func (p Platform) Valid() bool { return p == PlatformIOS || p == PlatformAndroid }

type Shape string

const (
	ShapeRect   Shape = "rect"
	ShapeCircle Shape = "circle"
)

func (s Shape) Valid() bool { return s == ShapeRect || s == ShapeCircle }

type Venue struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Timezone string  `json:"timezone"`
	JoinCode *string `json:"join_code"`
}

// ForRole hides the join code from masters, tech.md section 19.1.
func (v Venue) ForRole(r Role) Venue {
	if !r.CanManage() {
		v.JoinCode = nil
	}
	return v
}

type Hall struct {
	ID        string `json:"id"`
	VenueID   string `json:"venue_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type Staff struct {
	ID          string     `json:"id"`
	VenueID     string     `json:"venue_id"`
	DisplayName string     `json:"display_name"`
	Role        Role       `json:"role"`
	Blocked     bool       `json:"blocked"`
	CreatedAt   time.Time  `json:"created_at"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
}

type Table struct {
	ID       string `json:"id"`
	HallID   string `json:"hall_id"`
	Label    string `json:"label"`
	Shape    Shape  `json:"shape"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	W        int    `json:"w"`
	H        int    `json:"h"`
	Rotation int    `json:"rotation"`
	Seats    int    `json:"seats"`
}

// ValidUUID checks the canonical 8-4-4-4-12 hex form. Services treat a
// malformed id as not found instead of letting Postgres reject the cast.
func ValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHex(c) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

type Tokens struct {
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
}

// DecideClaim applies the claim rules of tech.md section 19.9. take is false
// when another master already holds the task, which is not an error.
func DecideClaim(hookah HookahState, ev ServiceEvent) (take bool, err error) {
	if hookah == HookahFinished {
		return false, ErrHookahFinished
	}
	if ev.State != EventDue {
		return false, ErrEventNotActionable
	}
	return ev.ClaimedBy == nil, nil
}
