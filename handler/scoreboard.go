package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Gigamons/Kokoro/helper"
	"github.com/Gigamons/common/consts"
	"github.com/Gigamons/common/helpers"
	"github.com/Gigamons/common/logger"
	"github.com/Gigamons/common/tools/usertools"
)

type Scoreboard struct {
	User              *consts.User
	ScoreboardType    int8
	ScoreboardVersion int8
	Beatmap           *helper.DBBeatmap
	PlayMode          int8
	Mods              uint16
	Friends           []*uint32
	Scores            []*Score
	ScoreIDs          []*uint32
}

type Score struct {
	ScoreID   uint32
	UserID    uint32
	FileMD5   string
	ScoreMD5  string
	ReplayMD5 string
	Score     int32
	MaxCombo  uint16
	PlayMode  int8
	Mods      uint32
	Count300  uint16
	Count100  uint16
	Count50   uint16
	CountGeki uint16
	CountKatu uint16
	CountMiss uint16
	Date      time.Time
	Accuracy  float64
	PP        float32
}

func GETScoreboard(w http.ResponseWriter, r *http.Request) {
	FileMD5 := r.URL.Query().Get("c")
	pm, err := strconv.Atoi(r.URL.Query().Get("m"))
	if err != nil {
		logger.Debugln("Error while parsing mode")
		fmt.Fprintf(w, "%v|false", consts.LatestPending)
		return
	}
	sbt, err := strconv.Atoi(r.URL.Query().Get("v"))
	if err != nil {
		logger.Debugln("Error while parsing Scoreboard Type")
		fmt.Fprintf(w, "%v|false", consts.LatestPending)
		return
	}
	sbv, err := strconv.Atoi(r.URL.Query().Get("vv"))
	if err != nil {
		logger.Debugln("Error while parsing Scoreboard Version")
		fmt.Fprintf(w, "%v|false", consts.LatestPending)
		return
	}
	mods, err := strconv.Atoi(r.URL.Query().Get("mods"))
	if err != nil {
		logger.Debugln("Error while parsing Mods")
		fmt.Fprintf(w, "%v|false", consts.LatestPending)
		return
	}
	UserID := usertools.GetUserID(r.URL.Query().Get("us"))
	if UserID < 0 {
		logger.Debugln("User Doesn't exists")
		return
	}
	User := usertools.GetUser(UserID)
	if User == nil {
		logger.Debugln("User Doesn't exists")
		return
	}
	if !User.CheckPassword(r.URL.Query().Get("ha")) {
		logger.Debugln("User exists, but Password doesn't match.")
		return
	}

	logger.Debugf(fmt.Sprintf("playMode%vbeatmap%sUser%v%sScoreboard%v%v%v%v", pm, FileMD5, User.ID, User.UserName, sbv, sbt, mods, pm))

	if cache, err := GetCache(fmt.Sprintf("playMode%vbeatmap%sUser%v%sScoreboard%v%v%v%v", pm, FileMD5, User.ID, User.UserName, sbv, sbt, mods, pm)); err == nil && cache != nil && len(cache) > 2 {
		w.Write(cache)
		return
	} else if err != nil {
		logger.Errorln(err)
	}

	bm := helper.GetBeatmapofDBHash(FileMD5)
	if bm == nil {
		_Cheese := helper.CheeseGull{}
		BM := _Cheese.GetBeatmapByHash(FileMD5)
		if BM == nil {
			logger.Debugln("Beatmap not found.")
			fmt.Fprintf(w, "%v|false", consts.LatestPending)
			return
		}
		Set := _Cheese.GetSet(int(BM.ParentSetID))
		if Set == nil {
			logger.Debugln("BeatmapSet not found.")
			fmt.Fprintf(w, "%v|false", consts.LatestPending)
			return
		}

		helper.AddBeatmap(Set)
		bm = helper.GetBeatmapofDBHash(FileMD5)
	}

	if bm == nil {
		return
	}

	ScoreBoard := Scoreboard{Beatmap: bm, User: User, ScoreboardVersion: int8(sbv), Mods: uint16(mods), PlayMode: int8(pm), ScoreboardType: int8(sbt)}
	output := ScoreBoard.DisplayScoreboard()
	SetCache(fmt.Sprintf("playMode%vbeatmap%sUser%v%sScoreboard%v%v%v%v", pm, FileMD5, User.ID, User.UserName, sbv, sbt, mods, pm), []byte(output), 120)
	w.Write([]byte(output))
}

func (sb *Scoreboard) DisplayScoreboard() (out string) {
	sb._SetScoreIDs()
	if len(sb.ScoreIDs) > 0 {
		sb._SetPersonalBest()
		sb._SetScores()
	}

	out += sb.Beatmap.GetHeader(len(sb.ScoreIDs))

	for i := 0; i < len(sb.Scores); i++ {
		s := sb.Scores[i]
		if s == nil {
			out += "\n"
			return
		}
		sowner := usertools.GetUser(int(s.UserID))
		fc := func() string {
			if s.CountMiss > 0 {
				return "False"
			}
			return "True"
		}()
		HasReplay := func() int {
			if s.ReplayMD5 != "" {
				return 1
			}
			return 0
		}()
		out += fmt.Sprintf("%v|%s|%v|%v|%v|%v|%v|%v|%v|%v|%s|%v|%v|%v|%v|%v\n", s.ScoreID, strings.Replace(sowner.UserName, "|", "I", -1), s.Score, s.MaxCombo, s.Count50, s.Count100, s.Count300, s.CountGeki, s.CountMiss, s.CountKatu, fc, s.Mods, s.UserID, s.Position(), s.Date.Unix(), HasReplay)
	}
	return
}

func (sb *Scoreboard) _SetPersonalBest() {
	QueryString := "SELECT * FROM scores WHERE UserID = ? AND FileMD5 = ? AND PlayMode = ? "

	if sb.Mods&128 > 0 || sb.Mods&8192 > 0 {
		QueryString += "AND (Mods & 128 != 0 OR Mods & 8192 != 0) "
	} else {
		QueryString += "AND (Mods & 128 = 0 AND Mods & 8192 = 0) "
	}
	QueryString += " ORDER BY score DESC LIMIT 1"

	ScoreRows, err := helpers.DB.Query(QueryString, sb.User.ID, sb.Beatmap.FileMD5, sb.PlayMode)
	if err != nil {
		logger.Errorln(err)
		return
	}
	i := 0
	for ScoreRows.Next() {
		i++
		score := &Score{}
		tmp := ""
		err := ScoreRows.Scan(
			&score.ScoreID,
			&score.UserID,
			&score.FileMD5,
			&score.ScoreMD5,
			&score.ReplayMD5,
			&score.Score,
			&score.MaxCombo,
			&score.PlayMode,
			&score.Mods,
			&score.Count300,
			&score.Count100,
			&score.Count50,
			&score.CountGeki,
			&score.CountKatu,
			&score.CountMiss,
			&tmp,
			&score.Accuracy,
			&score.PP,
		)
		if err != nil {
			logger.Errorln(err)
			continue
		}
		Date, err := time.Parse("2006-01-02 15:04:05", tmp)
		if err != nil {
			logger.Errorln(err)
			continue
		}
		score.Date = Date
		sb.Scores = append(sb.Scores, score)
	}
	if i == 0 {
		sb.Scores = append(sb.Scores, nil)
	}
}

func (s *Score) Position() int {
	Pos := 0
	rows, err := helpers.DB.Query("SELECT (SELECT COUNT(1) AS num FROM scores WHERE scores.Score > s1.Score AND FileMD5 = ?) + 1 AS rank FROM scores AS s1 WHERE FileMD5 = ? AND UserID = ? ORDER BY rank desc", s.FileMD5, s.FileMD5, s.UserID)
	if err != nil {
		logger.Errorln(err)
		return 0
	}
	for rows.Next() {
		if err := rows.Scan(&Pos); err != nil {
			logger.Errorln(err)
		}
	}
	return Pos
}

func antiInject(s string, arguments ...string) string {
	o := s
	for i := 0; i < len(arguments); i++ {
		o = strings.Replace(o, "?", "'"+strings.Replace(arguments[i], "'", "\\'", -1)+"'", 1)
	}
	return o
}

func (sb *Scoreboard) _SetScores() {
	QueryString := "SELECT * FROM scores WHERE ScoreID IN (" + inClause(len(sb.ScoreIDs)) + ")"

	ScoreRows, err := helpers.DB.Query(QueryString, uInt32ToSInterface(sb.ScoreIDs)...)
	if err != nil {
		logger.Errorln(err)
		return
	}

	for ScoreRows.Next() {
		score := &Score{}
		tmp := ""
		err := ScoreRows.Scan(
			&score.ScoreID,
			&score.UserID,
			&score.FileMD5,
			&score.ScoreMD5,
			&score.ReplayMD5,
			&score.Score,
			&score.MaxCombo,
			&score.PlayMode,
			&score.Mods,
			&score.Count300,
			&score.Count100,
			&score.Count50,
			&score.CountGeki,
			&score.CountKatu,
			&score.CountMiss,
			&tmp,
			&score.Accuracy,
			&score.PP,
		)
		if err != nil {
			logger.Errorln(err)
			continue
		}
		Date, err := time.Parse("2006-01-02 15:04:05", tmp)
		if err != nil {
			logger.Errorln(err)
			continue
		}
		score.Date = Date
		sb.Scores = append(sb.Scores, score)
	}

}

func (sb *Scoreboard) _SetScoreIDs() {
	sb._SetFriends()
	QueryString := "SELECT ScoreID, MAX(scores.Score) FROM scores STRAIGHT_JOIN users ON scores.UserID = users.id STRAIGHT_JOIN users_status ON users.id = users_status.id WHERE scores.FileMD5 = ? AND scores.PlayMode = ? AND (users_status.banned < 1 OR users.id = ?) "
	if sb.ScoreboardType == 4 {
		QueryString += "AND users_status.country = (SELECT country FROM users_status WHERE id = ? LIMIT 1) "
	}
	if sb.ScoreboardType == 2 {
		QueryString += "AND scores.Mods = ? "
	}
	if sb.ScoreboardType == 3 {
		QueryString += "AND (scores.UserID IN (SELECT friendid FROM friends WHERE userid = ?) OR scores.UserID = ?) "
	}
	if sb.Mods&128 > 0 || sb.Mods&8192 > 0 {
		QueryString += "AND (scores.Mods & 128 != 0 OR scores.Mods & 8192 != 0) "
	} else {
		QueryString += "AND (scores.Mods & 128 = 0 AND scores.Mods & 8192 = 0) "
	}
	QueryString += "GROUP BY UserID ORDER BY Score DESC LIMIT 100"

	Query, err := helpers.DB.Query(antiInject(QueryString, sb.Beatmap.FileMD5, strconv.Itoa(int(sb.PlayMode)), strconv.Itoa(int(sb.User.ID)), strconv.Itoa(int(sb.User.ID)), strconv.Itoa(int(sb.Mods)), strconv.Itoa(int(sb.User.ID)), strconv.Itoa(int(sb.User.ID))))
	if err != nil {
		logger.Errorln(err)
		return
	}
	for Query.Next() {
		var s uint32
		var t uint32
		if err := Query.Scan(&s, &t); err != nil {
			logger.Errorln(err)
		} else {
			sb.ScoreIDs = append(sb.ScoreIDs, &s)
		}
	}
}

func (sb *Scoreboard) _SetFriends() {
	var res []*uint32
	FriendList, err := helpers.DB.Query("SELECT friendid FROM friends WHERE userid = ?", sb.User.ID)
	if err != nil {
		return
	}
	for FriendList.Next() {
		var i uint32
		if err := FriendList.Scan(&i); err != nil {
			logger.Errorln(err)
		} else {
			res = append(res, &i)
		}
	}
	sb.Friends = res
}

// inClause, function by thehowl  Under MIT License. LINK https://github.com/osuripple/cheesegull
func inClause(length int) string {
	if length <= 0 {
		return ""
	}
	b := make([]byte, length*3-2)
	for i := 0; i < length; i++ {
		b[i*3] = '?'
		if i != length-1 {
			b[i*3+1] = ','
			b[i*3+2] = ' '
		}
	}
	return string(b)
}

// uInt32ToSInterface, function by thehowl  Under MIT License. LINK https://github.com/osuripple/cheesegull
func uInt32ToSInterface(i []*uint32) []interface{} {
	args := make([]interface{}, len(i))
	for idx, id := range i {
		args[idx] = id
	}
	return args
}
