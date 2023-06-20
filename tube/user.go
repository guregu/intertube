package tube

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	tableUsers   = "Users"
	counterUsers = "Users"
)

var TrialDuration = 14 * time.Hour * 24

const CurrentPhase = RegPhaseEarlyAccess

type User struct {
	ID       int
	Email    string
	Password []byte `json:"-"`
	Regdate  time.Time
	Phase    RegPhase // phase at time of reg
	Recovery string

	Usage  int64
	Quota  int64
	Tracks int

	CustomerID string // from stripe
	Plan       PlanKind
	PlanStatus PlanStatus // from stripe
	PlanExpire time.Time  `dynamo:",omitempty"`
	Canceled   bool
	TrialOver  bool

	Theme   string
	Display DisplayOptions

	B2Token  string
	B2Expire time.Time `dynamo:",omitempty"`

	LastMod  time.Time
	LastDump time.Time `dynamo:",omitempty"`
}

type DisplayOptions struct {
	Stretch     bool
	MusicLink   MusicLinkOpt
	TrackSelect TrackSelOpt
}

func (u *User) Create(ctx context.Context) error {
	if u.ID != 0 {
		panic("trying to create pre-existing user")
	}
	now := time.Now().UTC()
	u.Regdate = now
	u.Phase = CurrentPhase
	u.PlanExpire = now.Add(TrialDuration)
	u.Plan = PlanKindNone
	u.PlanStatus = PlanStatusTrialing
	u.Email = strings.ToLower(u.Email)

	var err error
	u.ID, err = NextID(ctx, counterUsers)
	if err != nil {
		return err
	}

	users := dynamoTable(tableUsers)
	err = users.Put(u).If("attribute_not_exists(ID)").Run()
	return err
}

func (u *User) SetEmail(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("blank email")
	}

	email = strings.ToLower(email)

	_, err := GetUserByEmail(ctx, email)
	if err == nil {
		return fmt.Errorf("e-mail already in use: %s", email)
	}
	if err != ErrNotFound {
		return err
	}

	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Email", email).
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u *User) SetRandomRecovery(ctx context.Context) error {
	code, err := randomString(69)
	if err != nil {
		return err
	}
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Recovery", code).
		Value(u)
}

func (u *User) SetPassword(ctx context.Context, pw []byte) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Password", pw).
		Remove("Recovery").
		Value(u)
}

func (u *User) UpdateLastMod(ctx context.Context) error {
	now := time.Now().UTC().Truncate(time.Second)
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("LastMod", now).
		Value(u)
}

func (u *User) UpdateLastDump(ctx context.Context, at time.Time) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("LastDump", at).
		If("attribute_not_exists(LastDump) OR LastDump < ?", at).
		Value(u)
}

func (u *User) SetB2Token(ctx context.Context, token string, expire time.Time) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("B2Token", token).
		Set("B2Expire", expire).
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u *User) SetTracks(ctx context.Context, count int) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Tracks", count).
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u *User) SetTheme(ctx context.Context, theme string) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Theme", theme).
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u *User) SetDisplayOpt(ctx context.Context, disp DisplayOptions) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Display", disp).
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u *User) SetCustomerID(ctx context.Context, id string) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("CustomerID", id).
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u *User) SetPlan(ctx context.Context, kind PlanKind, status PlanStatus, expires time.Time, canceled bool) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Set("Plan", kind).
		Set("PlanStatus", status).
		Set("PlanExpire", expires).
		Set("LastMod", time.Now().UTC()).
		Set("Canceled", canceled).
		Set("TrialOver", true).
		Value(u)
}

func (u *User) DeletePaymentInfo(ctx context.Context) error {
	users := dynamoTable(tableUsers)
	return users.Update("ID", u.ID).
		Remove("CustomerID").
		Remove("Plan").
		Remove("PlanStatus").
		Remove("PlanExpire").
		Remove("Canceled").
		Remove("TrialOver").
		Set("LastMod", time.Now().UTC()).
		Value(u)
}

func (u User) ValidPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(u.Password, []byte(password)) == nil
}

func (u User) CalcQuota() int64 {
	if u.Grandfathered() && u.Plan == PlanKindNone && u.Quota > 0 {
		return u.Quota
	}
	return GetPlan(u.Plan).Quota
}

func (u User) UsageDesc() string {
	if u.Usage == 0 {
		return "0"
	}
	if u.CalcQuota() == 0 {
		return "unlimited"
	}
	pct := float64(u.Usage) / float64(u.CalcQuota()) * 100
	return fmt.Sprintf("%.2f", pct)
}

func (u User) Active() bool {
	// TODO
	if u.Grandfathered() {
		return true
	}
	if !u.PlanStatus.Active() {
		return false
	}
	return u.PlanExpire.After(time.Now())
}

func (u User) TimeRemaining() time.Duration {
	if u.PlanExpire.IsZero() {
		return 0
	}
	return u.PlanExpire.Sub(time.Now())
}

func (u User) Expired() bool {
	return u.TimeRemaining() <= 0
}

func (u User) Grandfathered() bool {
	return u.Phase == RegPhaseAlpha
}

func (u User) Trialing() bool {
	return u.Plan == PlanKindNone
}

func (u User) StorageFull() bool {
	return u.Usage >= u.CalcQuota()
}

func GetUser(ctx context.Context, id int) (User, error) {
	users := dynamoTable(tableUsers)
	var u User
	err := users.Get("ID", id).Consistent(true).One(&u)
	return u, err
}

func GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(email)
	users := dynamoTable(tableUsers)
	var u User
	err := users.Get("Email", email).Index("Email-index").One(&u)
	return u, err
}

func GetUserByCustomerID(ctx context.Context, id string) (User, error) {
	users := dynamoTable(tableUsers)
	var u User
	err := users.Get("CustomerID", id).Index("CustomerID-index").One(&u)
	return u, err
}

func GetAllUsers(ctx context.Context) ([]User, error) {
	users := dynamoTable(tableUsers)
	var u []User
	err := users.Scan().All(&u)
	return u, err
}

func AddUsage(ctx context.Context, id int, usage int64, trackCt int) (User, error) {
	users := dynamoTable(tableUsers)
	var u User
	err := users.Update("ID", id).
		Add("Usage", usage).
		Add("Tracks", trackCt).
		If("attribute_exists('ID')").Value(&u)
	return u, err
}

func HashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

type MusicLinkOpt string

const (
	MusicLinkDefault MusicLinkOpt = ""
	MusicLinkAlbums  MusicLinkOpt = "albums"
)

func (mlo MusicLinkOpt) String() string {
	return string(mlo)
}

type TrackSelOpt string

const (
	TrackSelDefault TrackSelOpt = ""
	TrackSelCtrlKey TrackSelOpt = "ctrl"
)

func (tso TrackSelOpt) String() string {
	return string(tso)
}

type RegPhase string

const (
	RegPhaseAlpha       RegPhase = ""
	RegPhaseEarlyAccess RegPhase = "EA"
)
