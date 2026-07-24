package tlate

import (
	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/fileid"
)

func FillMedia(bm *botapi.Message, m telegram.MessageMedia) {
	switch v := m.(type) {
	case *telegram.MessageMediaPhoto:
		if p, ok := v.Photo.(*telegram.PhotoObj); ok {
			bm.Photo = photoSizes(p)
		}
	case *telegram.MessageMediaDocument:
		if d, ok := v.Document.(*telegram.DocumentObj); ok {
			fillFromDocument(bm, d)
		}
	case *telegram.MessageMediaContact:
		bm.Contact = &botapi.Contact{
			PhoneNumber: v.PhoneNumber,
			FirstName:   v.FirstName,
			LastName:    v.LastName,
			UserID:      v.UserID,
			VCard:       v.Vcard,
		}
	case *telegram.MessageMediaGeo:
		if pt, ok := v.Geo.(*telegram.GeoPointObj); ok {
			bm.Location = &botapi.Location{
				Latitude:           pt.Lat,
				Longitude:          pt.Long,
				HorizontalAccuracy: float64(pt.AccuracyRadius),
			}
		}
	case *telegram.MessageMediaGeoLive:
		if pt, ok := v.Geo.(*telegram.GeoPointObj); ok {
			bm.Location = &botapi.Location{
				Latitude:             pt.Lat,
				Longitude:            pt.Long,
				HorizontalAccuracy:   float64(pt.AccuracyRadius),
				LivePeriod:           int(v.Period),
				Heading:              int(v.Heading),
				ProximityAlertRadius: int(v.ProximityNotificationRadius),
			}
		}
	case *telegram.MessageMediaVenue:
		var loc botapi.Location
		if pt, ok := v.Geo.(*telegram.GeoPointObj); ok {
			loc = botapi.Location{Latitude: pt.Lat, Longitude: pt.Long}
		}
		bm.Venue = &botapi.Venue{
			Location:       loc,
			Title:          v.Title,
			Address:        v.Address,
			FoursquareID:   v.VenueID,
			FoursquareType: v.VenueType,
		}
	case *telegram.MessageMediaDice:
		bm.Dice = &botapi.Dice{Emoji: v.Emoticon, Value: int(v.Value)}
	case *telegram.MessageMediaPoll:
		if v.Poll != nil {
			bm.Poll = pollToBotAPI(v.Poll, v.Results)
		}
	}
}

func photoSizes(p *telegram.PhotoObj) []botapi.PhotoSize {
	out := make([]botapi.PhotoSize, 0, len(p.Sizes))
	for _, s := range p.Sizes {
		var ps botapi.PhotoSize
		var sizeType string
		switch v := s.(type) {
		case *telegram.PhotoSizeObj:
			ps = botapi.PhotoSize{Width: int(v.W), Height: int(v.H), FileSize: int64(v.Size)}
			sizeType = v.Type
		case *telegram.PhotoCachedSize:
			ps = botapi.PhotoSize{Width: int(v.W), Height: int(v.H), FileSize: int64(len(v.Bytes))}
			sizeType = v.Type
		case *telegram.PhotoStrippedSize:
			continue
		case *telegram.PhotoSizeProgressive:
			total := int64(0)
			if len(v.Sizes) > 0 {
				total = int64(v.Sizes[len(v.Sizes)-1])
			}
			ps = botapi.PhotoSize{Width: int(v.W), Height: int(v.H), FileSize: total}
			sizeType = v.Type
		default:
			continue
		}
		info := &fileid.Info{
			DC:         p.DcID,
			Type:       fileid.FTPhoto,
			ID:         p.ID,
			AccessHash: p.AccessHash,
			FileRef:    p.FileReference,
			PhotoSize:  sizeType,
		}
		ps.FileID = info.Encode()
		ps.FileUniqueID = info.UniqueID()
		out = append(out, ps)
	}
	return out
}

func fillFromDocument(bm *botapi.Message, d *telegram.DocumentObj) {
	var (
		isAudio, isVoice, isVideo, isVideoNote, isSticker, isAnimation bool
		fileName, performer, title                                     string
		duration, width, height, length                                int
	)
	for _, a := range d.Attributes {
		switch v := a.(type) {
		case *telegram.DocumentAttributeAudio:
			isAudio = true
			duration = int(v.Duration)
			performer = v.Performer
			title = v.Title
			if v.Voice {
				isVoice = true
			}
		case *telegram.DocumentAttributeVideo:
			isVideo = true
			duration = int(v.Duration)
			width = int(v.W)
			height = int(v.H)
			if v.RoundMessage {
				isVideoNote = true
				length = width
			}
		case *telegram.DocumentAttributeSticker:
			isSticker = true
		case *telegram.DocumentAttributeAnimated:
			isAnimation = true
		case *telegram.DocumentAttributeFilename:
			fileName = v.FileName
		case *telegram.DocumentAttributeImageSize:
			width = int(v.W)
			height = int(v.H)
		}
	}

	ft := fileid.FTDocument
	switch {
	case isVoice:
		ft = fileid.FTVoiceNote
	case isVideoNote:
		ft = fileid.FTVideoNote
	case isSticker:
		ft = fileid.FTSticker
	case isAnimation:
		ft = fileid.FTAnimation
	case isVideo:
		ft = fileid.FTVideo
	case isAudio:
		ft = fileid.FTAudio
	}
	info := &fileid.Info{
		DC:         d.DcID,
		Type:       ft,
		ID:         d.ID,
		AccessHash: d.AccessHash,
		FileRef:    d.FileReference,
	}
	fid := info.Encode()
	uid := info.UniqueID()
	var thumb *botapi.PhotoSize
	for _, s := range d.Thumbs {
		if ps, ok := s.(*telegram.PhotoSizeObj); ok {
			thumb = &botapi.PhotoSize{Width: int(ps.W), Height: int(ps.H), FileSize: int64(ps.Size)}
			thumbInfo := &fileid.Info{
				DC:         d.DcID,
				Type:       fileid.FTThumbnail,
				ID:         d.ID,
				AccessHash: d.AccessHash,
				FileRef:    d.FileReference,
				PhotoSize:  ps.Type,
			}
			thumb.FileID = thumbInfo.Encode()
			thumb.FileUniqueID = thumbInfo.UniqueID()
			break
		}
	}

	switch {
	case isVoice:
		bm.Voice = &botapi.Voice{FileID: fid, FileUniqueID: uid, Duration: duration, MimeType: d.MimeType, FileSize: d.Size}
	case isVideoNote:
		bm.VideoNote = &botapi.VideoNote{FileID: fid, FileUniqueID: uid, Duration: duration, Length: length, Thumbnail: thumb, FileSize: d.Size}
	case isSticker:
		bm.Sticker = &botapi.Sticker{
			FileID: fid, FileUniqueID: uid, Type: "regular",
			Width: width, Height: height, Thumbnail: thumb, FileSize: d.Size,
		}
	case isAnimation:
		bm.Animation = &botapi.Animation{
			FileID: fid, FileUniqueID: uid, Duration: duration, Width: width, Height: height,
			Thumbnail: thumb, FileName: fileName, MimeType: d.MimeType, FileSize: d.Size,
		}
	case isVideo:
		bm.Video = &botapi.Video{
			FileID: fid, FileUniqueID: uid, Duration: duration, Width: width, Height: height,
			Thumbnail: thumb, FileName: fileName, MimeType: d.MimeType, FileSize: d.Size,
		}
	case isAudio:
		bm.Audio = &botapi.Audio{
			FileID: fid, FileUniqueID: uid, Duration: duration, Performer: performer, Title: title,
			FileName: fileName, MimeType: d.MimeType, FileSize: d.Size, Thumbnail: thumb,
		}
	default:
		bm.Document = &botapi.Document{
			FileID: fid, FileUniqueID: uid, Thumbnail: thumb, FileName: fileName,
			MimeType: d.MimeType, FileSize: d.Size,
		}
	}
}

func pollToBotAPI(p *telegram.Poll, r *telegram.PollResults) *botapi.Poll {
	bp := &botapi.Poll{
		ID:                    itoa64(p.ID),
		Question:              p.Question.Text,
		IsClosed:              p.Closed,
		IsAnonymous:           !p.PublicVoters,
		AllowsMultipleAnswers: p.MultipleChoice,
		OpenPeriod:            int(p.ClosePeriod),
		CloseDate:             int64(p.CloseDate),
	}
	if p.Quiz {
		bp.Type = "quiz"
	} else {
		bp.Type = "regular"
	}
	for _, opt := range p.Answers {
		if o, ok := opt.(*telegram.PollAnswerObj); ok && o.Text != nil {
			bp.Options = append(bp.Options, botapi.PollOption{Text: o.Text.Text})
		}
	}
	if r != nil {
		bp.TotalVoterCount = int(r.TotalVoters)
		for i, res := range r.Results {
			if i < len(bp.Options) {
				bp.Options[i].VoterCount = int(res.Voters)
			}
		}
	}
	return bp
}
