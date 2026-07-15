package race

import (
	"encoding/base64"
	"encoding/xml"
	"net/http"
	"owfc/common"
	"owfc/logging"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

type rankingsRequestEnvelope struct {
	Body rankingsRequestBody
}

type rankingsRequestBody struct {
	Data rankingsRequestData `xml:",any"`
}

type rankingsRequestData struct {
	XMLName  xml.Name
	GameId   int                                    `xml:"gameid"`
	RegionId common.MarioKartWiiLeaderboardRegionId `xml:"regionid"`
	CourseId common.MarioKartWiiCourseId            `xml:"courseid"`
}

type rankingsResponseRankingDataResponse struct {
	XMLName      xml.Name          `xml:"RankingDataResponse"`
	XMLNSXSI     string            `xml:"xmlns:xsi,attr"`
	XMLNSXSD     string            `xml:"xmlns:xsd,attr"`
	XMLNS        string            `xml:"xmlns,attr"`
	ResponseCode raceServiceResult `xml:"responseCode"`
	DataArray    rankingsResponseDataArray
}

type rankingsResponseDataArray struct {
	XMLName    xml.Name `xml:"dataArray"`
	NumRecords int      `xml:"numrecords"`
	Data       []rankingsResponseData
}

type rankingsResponseData struct {
	XMLName     xml.Name `xml:"data"`
	RankingData rankingsResponseRankingData
}

type rankingsResponseRankingData struct {
	XMLName  xml.Name `xml:"RankingData"`
	OwnerID  int      `xml:"ownerid"`
	Rank     int      `xml:"rank"`
	Time     int      `xml:"time"`
	UserData string   `xml:"userdata"`
}

type raceServiceResult int

// https://github.com/GameProgressive/UniSpySDK/blob/master/webservices/RacingService.h
const (
	raceServiceResultSuccess           = 0
	raceServiceResultDatabaseError     = 6
	raceServiceResultParseError        = 101
	raceServiceResultInvalidParameters = 105
)

const (
	xmlNamespaceXSI = "http://www.w3.org/2001/XMLSchema-instance"
	xmlNamespaceXSD = "http://www.w3.org/2001/XMLSchema"
	xmlNamespace    = "http://gamespy.net/RaceService/"
)

var marioKartWiiGameID = 1687

func handleNintendoRacingServiceRequest(w http.ResponseWriter, r *http.Request) {
	moduleName := "RACE:RacingService:" + r.RemoteAddr

	soapActionHeader := r.Header.Get("SOAPAction")
	if soapActionHeader == "" {
		logging.Error(moduleName, "No SOAPAction header")
		writeErrorResponse(raceServiceResultParseError, w)
		return
	}

	slashIndex := strings.LastIndex(soapActionHeader, "/")
	if slashIndex == -1 {
		logging.Error(moduleName, "Invalid SOAPAction header")
		writeErrorResponse(raceServiceResultParseError, w)
		return
	}
	quotationMarkIndex := strings.Index(soapActionHeader[slashIndex+1:], "\"")
	if quotationMarkIndex == -1 {
		logging.Error(moduleName, "Invalid SOAPAction header")
		writeErrorResponse(raceServiceResultParseError, w)
		return
	}

	soapAction := soapActionHeader[slashIndex+1 : slashIndex+1+quotationMarkIndex]
	switch soapAction {
	case "GetTopTenRankings":
		handleGetTopTenRankingsRequest(moduleName, w, r)

	// TODO SubmitScores
	default:
		logging.Info(moduleName, "Unhandled SOAPAction:", aurora.Cyan(soapAction))
	}
}

func handleGetTopTenRankingsRequest(moduleName string, w http.ResponseWriter, r *http.Request) {
	var request rankingsRequestEnvelope
	err := xml.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		logging.Error(moduleName, "Got malformed XML")
		writeErrorResponse(raceServiceResultParseError, w)
		return
	}

	requestData := request.Body.Data

	gameId := requestData.GameId
	if gameId != marioKartWiiGameID {
		logging.Error(moduleName, "Wrong GameSpy game ID:", aurora.Cyan(gameId))
		writeErrorResponse(raceServiceResultInvalidParameters, w)
		return
	}

	regionId := requestData.RegionId
	courseId := requestData.CourseId

	if !regionId.IsValid() {
		logging.Error(moduleName, "Invalid region ID:", aurora.Cyan(regionId))
		writeErrorResponse(raceServiceResultInvalidParameters, w)
		return
	}
	if courseId < common.MarioCircuit || courseId > 32767 {
		logging.Error(moduleName, "Invalid course ID:", aurora.Cyan(courseId))
		writeErrorResponse(raceServiceResultInvalidParameters, w)
		return
	}

	topTenRankings, err := db.GetMarioKartWiiTopTenRankings(regionId, courseId)
	if err != nil {
		logging.Error(moduleName, "Failed to get the Top 10 rankings:", err)
		writeErrorResponse(raceServiceResultDatabaseError, w)
		return
	}

	numberOfRankings := len(topTenRankings)
	data := make([]rankingsResponseData, 0, numberOfRankings)
	for i, topTenRanking := range topTenRankings {
		// Filter player info just in case
		playerInfo, err := base64.StdEncoding.DecodeString(topTenRanking.PlayerInfo)
		if err != nil {
			panic(err)
		}
		miiData := common.RawMiiFromBytes(playerInfo).ClearMiiInfo().Data
		playerInfo = append(miiData[:], playerInfo[len(miiData):]...)

		rankingData := rankingsResponseRankingData{
			OwnerID:  topTenRanking.PID,
			Rank:     i + 1,
			Time:     topTenRanking.Score,
			UserData: base64.StdEncoding.EncodeToString(playerInfo),
		}

		responseData := rankingsResponseData{
			RankingData: rankingData,
		}

		data = append(data, responseData)
	}

	dataArray := rankingsResponseDataArray{
		NumRecords: numberOfRankings,
		Data:       data,
	}

	rankingDataResponse := rankingsResponseRankingDataResponse{
		XMLNSXSI:     xmlNamespaceXSI,
		XMLNSXSD:     xmlNamespaceXSD,
		XMLNS:        xmlNamespace,
		ResponseCode: raceServiceResultSuccess,
		DataArray:    dataArray,
	}

	writeResponse(w, rankingDataResponse)
}

func writeErrorResponse(raceServiceResult raceServiceResult, w http.ResponseWriter) {
	rankingDataResponse := rankingsResponseRankingDataResponse{
		XMLNSXSI:     xmlNamespaceXSI,
		XMLNSXSD:     xmlNamespaceXSD,
		XMLNS:        xmlNamespace,
		ResponseCode: raceServiceResult,
	}

	writeResponse(w, rankingDataResponse)
}

func writeResponse(w http.ResponseWriter, rankingDataResponse rankingsResponseRankingDataResponse) {
	w.Header().Set("Content-Type", "text/xml")
	w.Write([]byte(xml.Header))
	err := xml.NewEncoder(w).Encode(rankingDataResponse)
	if err != nil {
		logging.Error("Failed to write response:", err)
	}
}
