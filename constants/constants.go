package constants

import (
	"regexp"
	"strings"
)

const NONE = "none"
const NAME = "erodownloader"
const TMP_DIR = ".edtmp"
const ORIG_DIR = ".orig"
const LOCAL_CLIENT = "local"
const FILENAME_MAX_LENGTH = 240

// tmp files that are created or renamed to by download manager when downloading.
// E.g. ".aria2", ".!qB".
var IncompleteFileExts = []string{".aria2", ".!qB"}

// Private ip, e.g. 192.168.0.0/16, 127.0.0.0/8, etc.
// from https://stackoverflow.com/questions/2814002/private-ip-address-identifier-in-regular-expression .
var PrivateIpRegexp = regexp.MustCompile(`(^127\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$)|(^10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$)|(^172\.1[6-9]{1}[0-9]{0,1}\.[0-9]{1,3}\.[0-9]{1,3}$)|(^172\.2[0-9]{1}[0-9]{0,1}\.[0-9]{1,3}\.[0-9]{1,3}$)|(^172\.3[0-1]{1}[0-9]{0,1}\.[0-9]{1,3}\.[0-9]{1,3}$)|(^192\.168\.[0-9]{1,3}\.[0-9]{1,3}$)`)

// It's a subset of https://rclone.org/overview/#restricted-filenames-caveats .
// Only include invalid filename characters in Windows (NTFS).
var FilepathRestrictedCharacterReplacement = map[rune]rune{
	'*': '＊',
	':': '：',
	'<': '＜',
	'>': '＞',
	'|': '｜',
	'?': '？',
	'"': '＂',
}

var FilenameRestrictedCharacterReplacement = map[rune]rune{
	'/':  '／',
	'\\': '＼',
}

// Replace invalid Windows filename chars to alternatives. E.g. '/' => '／', 	'?' => '？'
var FilenameRestrictedCharacterReplacer *strings.Replacer

// Replace invalid Windows file path chars to alternatives.
// Similar to FilenameRestrictedCharacterReplacer, but do not replace '/' or '\'.
var FilepathRestrictedCharacterReplacer *strings.Replacer

// 0xEF, 0xBB, 0xBF
var Utf8bom = []byte{0xEF, 0xBB, 0xBF}

// common img exts: .webp, .png, .jpg...
var ImgExts = []string{".webp", ".png", ".jpg", ".jpeg"}

// common archive exts: ".rar", ".zip", ".7z"...
var ArchiveExts = []string{".rar", ".zip", ".7z"}

// common exts. exts for same type file are sorted by popularity order.
var CommonExts = []string{}

// In priority order.
var CjkCharsets = []string{
	"UTF-8",
	"Shift_JIS",
	"GB-18030",
	"EUC-KR",
	"EUC-JP",
	"Big5", //  !部分GBK字符串误识别为 Big5
}

func init() {
	CommonExts = append(CommonExts, ImgExts...)
	CommonExts = append(CommonExts, ArchiveExts...)

	args := []string{}
	for old, new := range FilepathRestrictedCharacterReplacement {
		args = append(args, string(old), string(new))
	}
	FilepathRestrictedCharacterReplacer = strings.NewReplacer(args...)
	for old, new := range FilenameRestrictedCharacterReplacement {
		args = append(args, string(old), string(new))
	}
	FilenameRestrictedCharacterReplacer = strings.NewReplacer(args...)
}
