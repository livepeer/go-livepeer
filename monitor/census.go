package monitor

import (
	"context"
	"math/big"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"contrib.go.opencensus.io/exporter/prometheus"
	rprom "github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

type (
	SegmentUploadError    string
	SegmentTranscodeError string
)

const (
	SegmentUploadErrorUnknown               SegmentUploadError    = "Unknown"
	SegmentUploadErrorGenCreds              SegmentUploadError    = "GenCreds"
	SegmentUploadErrorOS                    SegmentUploadError    = "ObjectStorage"
	SegmentUploadErrorSessionEnded          SegmentUploadError    = "SessionEnded"
	SegmentUploadErrorInsufficientBalance   SegmentUploadError    = "InsufficientBalance"
	SegmentUploadErrorTimeout               SegmentUploadError    = "Timeout"
	SegmentUploadErrorDuplicateSegment      SegmentUploadError    = "DuplicateSegment"
	SegmentUploadErrorOrchestratorCapped    SegmentUploadError    = "OrchestratorCapped"
	SegmentTranscodeErrorUnknown            SegmentTranscodeError = "Unknown"
	SegmentTranscodeErrorUnknownResponse    SegmentTranscodeError = "UnknownResponse"
	SegmentTranscodeErrorTranscode          SegmentTranscodeError = "Transcode"
	SegmentTranscodeErrorOrchestratorBusy   SegmentTranscodeError = "OrchestratorBusy"
	SegmentTranscodeErrorOrchestratorCapped SegmentTranscodeError = "OrchestratorCapped"
	SegmentTranscodeErrorParseResponse      SegmentTranscodeError = "ParseResponse"
	SegmentTranscodeErrorReadBody           SegmentTranscodeError = "ReadBody"
	SegmentTranscodeErrorNoOrchestrators    SegmentTranscodeError = "NoOrchestrators"
	SegmentTranscodeErrorDownload           SegmentTranscodeError = "Download"
	SegmentTranscodeErrorSaveData           SegmentTranscodeError = "SaveData"
	SegmentTranscodeErrorSessionEnded       SegmentTranscodeError = "SessionEnded"
	SegmentTranscodeErrorDuplicateSegment   SegmentTranscodeError = "DuplicateSegment"

	numberOfSegmentsToCalcAverage = 30
	gweiConversionFactor          = 1000000000

	logLevel = 6 // TODO move log levels definitions to separate package
	// importing `common` package here introduces import cycles
)

type NodeType string

const (
	Default      NodeType = "dflt"
	Orchestrator NodeType = "orch"
	Broadcaster  NodeType = "bctr"
	Transcoder   NodeType = "trcr"
	Redeemer     NodeType = "rdmr"

	segTypeRegular = "regular"
	segTypeRec     = "recorded" // segment in the stream for which recording is enabled
)

// Enabled true if metrics was enabled in command line
var Enabled bool

var NodeID string

var timeToWaitForError = 8500 * time.Millisecond
var timeoutWatcherPause = 15 * time.Second

type (
	censusMetricsCounter struct {
		nodeType                      NodeType
		nodeID                        string
		ctx                           context.Context
		kGPU                          tag.Key
		kNodeType                     tag.Key
		kNodeID                       tag.Key
		kProfile                      tag.Key
		kProfiles                     tag.Key
		kErrorCode                    tag.Key
		kTry                          tag.Key
		kSender                       tag.Key
		kRecipient                    tag.Key
		kManifestID                   tag.Key
		kSegmentType                  tag.Key
		kTrusted                      tag.Key
		kVerified                     tag.Key
		mSegmentSourceAppeared        *stats.Int64Measure
		mSegmentEmerged               *stats.Int64Measure
		mSegmentEmergedUnprocessed    *stats.Int64Measure
		mSegmentUploaded              *stats.Int64Measure
		mSegmentUploadFailed          *stats.Int64Measure
		mSegmentDownloaded            *stats.Int64Measure
		mSegmentTranscoded            *stats.Int64Measure
		mSegmentTranscodedUnprocessed *stats.Int64Measure
		mSegmentTranscodeFailed       *stats.Int64Measure
		mSegmentTranscodedAppeared    *stats.Int64Measure
		mSegmentTranscodedAllAppeared *stats.Int64Measure
		mStartBroadcastClientFailed   *stats.Int64Measure
		mStreamCreateFailed           *stats.Int64Measure
		mStreamCreated                *stats.Int64Measure
		mStreamStarted                *stats.Int64Measure
		mStreamEnded                  *stats.Int64Measure
		mMaxSessions                  *stats.Int64Measure
		mCurrentSessions              *stats.Int64Measure
		mDiscoveryError               *stats.Int64Measure
		mTranscodeRetried             *stats.Int64Measure
		mTranscodersNumber            *stats.Int64Measure
		mTranscodersCapacity          *stats.Int64Measure
		mTranscodersLoad              *stats.Int64Measure
		mSuccessRate                  *stats.Float64Measure
		mTranscodeTime                *stats.Float64Measure
		mTranscodeLatency             *stats.Float64Measure
		mTranscodeOverallLatency      *stats.Float64Measure
		mUploadTime                   *stats.Float64Measure
		mDownloadTime                 *stats.Float64Measure
		mAuthWebhookTime              *stats.Float64Measure
		mSourceSegmentDuration        *stats.Float64Measure
		mHTTPClientTimeout1           *stats.Int64Measure
		mHTTPClientTimeout2           *stats.Int64Measure
		mRealtime3x                   *stats.Int64Measure
		mRealtime2x                   *stats.Int64Measure
		mRealtime1x                   *stats.Int64Measure
		mRealtimeHalf                 *stats.Int64Measure
		mRealtimeSlow                 *stats.Int64Measure
		mTranscodeScore               *stats.Float64Measure
		mRecordingSaveLatency         *stats.Float64Measure
		mRecordingSaveErrors          *stats.Int64Measure
		mRecordingSavedSegments       *stats.Int64Measure
		mOrchestratorSwaps            *stats.Int64Measure

		// Metrics for sending payments
		mTicketValueSent     *stats.Float64Measure
		mTicketsSent         *stats.Int64Measure
		mPaymentCreateError  *stats.Int64Measure
		mDeposit             *stats.Float64Measure
		mReserve             *stats.Float64Measure
		mMaxTranscodingPrice *stats.Float64Measure
		// Metrics for receiving payments
		mTicketValueRecv       *stats.Float64Measure
		mTicketsRecv           *stats.Int64Measure
		mPaymentRecvErr        *stats.Int64Measure
		mWinningTicketsRecv    *stats.Int64Measure
		mValueRedeemed         *stats.Float64Measure
		mTicketRedemptionError *stats.Int64Measure
		mSuggestedGasPrice     *stats.Float64Measure
		mMinGasPrice           *stats.Float64Measure
		mMaxGasPrice           *stats.Float64Measure
		mTranscodingPrice      *stats.Float64Measure

		// Metrics for pixel accounting
		mMilPixelsProcessed *stats.Float64Measure

		// Metrics for fast verification
		mFastVerificationDone                   *stats.Int64Measure
		mFastVerificationFailed                 *stats.Int64Measure
		mFastVerificationEnabledCurrentSessions *stats.Int64Measure
		mFastVerificationUsingCurrentSessions   *stats.Int64Measure

		lock        sync.Mutex
		emergeTimes map[uint64]map[uint64]time.Time // nonce:seqNo
		success     map[uint64]*segmentsAverager
	}

	segmentCount struct {
		seqNo       uint64
		emergedTime time.Time
		emerged     int
		transcoded  int
		failed      bool
	}

	tryData struct {
		first time.Time
		tries int
	}

	segmentsAverager struct {
		segments  []segmentCount
		start     int
		end       int
		removed   bool
		removedAt time.Time
		tries     map[uint64]tryData // seqNo:try
	}
)

// Exporter Prometheus exporter that handles `/metrics` endpoint
var Exporter *prometheus.Exporter

var census censusMetricsCounter

// used in unit tests
var unitTestMode bool

func InitCensus(nodeType NodeType, version string) {
	census = censusMetricsCounter{
		emergeTimes: make(map[uint64]map[uint64]time.Time),
		nodeID:      NodeID,
		nodeType:    nodeType,
		success:     make(map[uint64]*segmentsAverager),
	}
	var err error
	ctx := context.Background()
	census.kGPU = tag.MustNewKey("gpu")
	census.kNodeType = tag.MustNewKey("node_type")
	census.kNodeID = tag.MustNewKey("node_id")
	census.kProfile = tag.MustNewKey("profile")
	census.kProfiles = tag.MustNewKey("profiles")
	census.kErrorCode = tag.MustNewKey("error_code")
	census.kTry = tag.MustNewKey("try")
	census.kSender = tag.MustNewKey("sender")
	census.kRecipient = tag.MustNewKey("recipient")
	census.kManifestID = tag.MustNewKey("manifestID")
	census.kSegmentType = tag.MustNewKey("seg_type")
	census.kTrusted = tag.MustNewKey("trusted")
	census.kVerified = tag.MustNewKey("verified")
	census.ctx, err = tag.New(ctx, tag.Insert(census.kNodeType, string(nodeType)), tag.Insert(census.kNodeID, NodeID))
	if err != nil {
		glog.Fatal("Error creating context", err)
	}
	census.mHTTPClientTimeout1 = stats.Int64("http_client_timeout_1", "Number of times HTTP connection was dropped before transcoding complete", "tot")
	census.mHTTPClientTimeout2 = stats.Int64("http_client_timeout_2", "Number of times HTTP connection was dropped before transcoded segments was sent back to client", "tot")
	census.mRealtime3x = stats.Int64("http_client_segment_transcoded_realtime_3x", "Number of segment transcoded 3x faster than realtime", "tot")
	census.mRealtime2x = stats.Int64("http_client_segment_transcoded_realtime_2x", "Number of segment transcoded 2x faster than realtime", "tot")
	census.mRealtime1x = stats.Int64("http_client_segment_transcoded_realtime_1x", "Number of segment transcoded 1x faster than realtime", "tot")
	census.mRealtimeHalf = stats.Int64("http_client_segment_transcoded_realtime_half", "Number of segment transcoded no more than two times slower than realtime", "tot")
	census.mRealtimeSlow = stats.Int64("http_client_segment_transcoded_realtime_slow", "Number of segment transcoded more than two times slower than realtime", "tot")
	census.mSegmentSourceAppeared = stats.Int64("segment_source_appeared_total", "SegmentSourceAppeared", "tot")
	census.mSegmentEmerged = stats.Int64("segment_source_emerged_total", "SegmentEmerged", "tot")
	census.mSegmentEmergedUnprocessed = stats.Int64("segment_source_emerged_unprocessed_total", "SegmentEmerged, counted by number of transcode profiles", "tot")
	census.mSegmentUploaded = stats.Int64("segment_source_uploaded_total", "SegmentUploaded", "tot")
	census.mSegmentUploadFailed = stats.Int64("segment_source_upload_failed_total", "SegmentUploadedFailed", "tot")
	census.mSegmentDownloaded = stats.Int64("segment_transcoded_downloaded_total", "SegmentDownloaded", "tot")
	census.mSegmentTranscoded = stats.Int64("segment_transcoded_total", "SegmentTranscoded", "tot")
	census.mSegmentTranscodedUnprocessed = stats.Int64("segment_transcoded_unprocessed_total", "SegmentTranscodedUnprocessed", "tot")
	census.mSegmentTranscodeFailed = stats.Int64("segment_transcode_failed_total", "SegmentTranscodeFailed", "tot")
	census.mSegmentTranscodedAppeared = stats.Int64("segment_transcoded_appeared_total", "SegmentTranscodedAppeared", "tot")
	census.mSegmentTranscodedAllAppeared = stats.Int64("segment_transcoded_all_appeared_total", "SegmentTranscodedAllAppeared", "tot")
	census.mStartBroadcastClientFailed = stats.Int64("broadcast_client_start_failed_total", "StartBroadcastClientFailed", "tot")
	census.mStreamCreateFailed = stats.Int64("stream_create_failed_total", "StreamCreateFailed", "tot")
	census.mStreamCreated = stats.Int64("stream_created_total", "StreamCreated", "tot")
	census.mStreamStarted = stats.Int64("stream_started_total", "StreamStarted", "tot")
	census.mStreamEnded = stats.Int64("stream_ended_total", "StreamEnded", "tot")
	census.mMaxSessions = stats.Int64("max_sessions_total", "MaxSessions", "tot")
	census.mCurrentSessions = stats.Int64("current_sessions_total", "Number of currently transcoded streams", "tot")
	census.mDiscoveryError = stats.Int64("discovery_errors_total", "Number of discover errors", "tot")
	census.mTranscodeRetried = stats.Int64("transcode_retried", "Number of times segment transcode was retried", "tot")
	census.mTranscodersNumber = stats.Int64("transcoders_number", "Number of transcoders currently connected to orchestrator", "tot")
	census.mTranscodersCapacity = stats.Int64("transcoders_capacity", "Total advertised capacity of transcoders currently connected to orchestrator", "tot")
	census.mTranscodersLoad = stats.Int64("transcoders_load", "Total load of transcoders currently connected to orchestrator", "tot")
	census.mSuccessRate = stats.Float64("success_rate", "Success rate", "per")
	census.mTranscodeTime = stats.Float64("transcode_time_seconds", "Transcoding time", "sec")
	census.mTranscodeLatency = stats.Float64("transcode_latency_seconds",
		"Transcoding latency, from source segment emerged from segmenter till transcoded segment apeeared in manifest", "sec")
	census.mTranscodeOverallLatency = stats.Float64("transcode_overall_latency_seconds",
		"Transcoding latency, from source segment emerged from segmenter till all transcoded segment apeeared in manifest", "sec")
	census.mUploadTime = stats.Float64("upload_time_seconds", "Upload (to Orchestrator) time", "sec")
	census.mDownloadTime = stats.Float64("download_time_seconds", "Download (from orchestrator) time", "sec")
	census.mAuthWebhookTime = stats.Float64("auth_webhook_time_milliseconds", "Authentication webhook execution time", "ms")
	census.mSourceSegmentDuration = stats.Float64("source_segment_duration_seconds", "Source segment's duration", "sec")
	census.mTranscodeScore = stats.Float64("transcode_score", "Ratio of source segment duration vs. transcode time", "rat")
	census.mRecordingSaveLatency = stats.Float64("recording_save_latency",
		"How long it takes to save segment to the OS", "sec")
	census.mRecordingSaveErrors = stats.Int64("recording_save_errors", "Number of errors during save to the recording OS", "tot")
	census.mRecordingSavedSegments = stats.Int64("recording_saved_segments", "Number of segments saved to the recording OS", "tot")
	census.mOrchestratorSwaps = stats.Int64("orchestrator_swaps", "Number of orchestrator swaps mid-stream", "tot")

	// Metrics for sending payments
	census.mTicketValueSent = stats.Float64("ticket_value_sent", "TicketValueSent", "gwei")
	census.mTicketsSent = stats.Int64("tickets_sent", "TicketsSent", "tot")
	census.mPaymentCreateError = stats.Int64("payment_create_errors", "PaymentCreateError", "tot")
	census.mDeposit = stats.Float64("broadcaster_deposit", "Current remaining deposit for the broadcaster node", "gwei")
	census.mReserve = stats.Float64("broadcaster_reserve", "Current remaing reserve for the broadcaster node", "gwei")
	census.mMaxTranscodingPrice = stats.Float64("max_transcoding_price", "MaxTranscodingPrice", "wei")

	// Metrics for receiving payments
	census.mTicketValueRecv = stats.Float64("ticket_value_recv", "TicketValueRecv", "gwei")
	census.mTicketsRecv = stats.Int64("tickets_recv", "TicketsRecv", "tot")
	census.mPaymentRecvErr = stats.Int64("payment_recv_errors", "PaymentRecvErr", "tot")
	census.mWinningTicketsRecv = stats.Int64("winning_tickets_recv", "WinningTicketsRecv", "tot")
	census.mValueRedeemed = stats.Float64("value_redeemed", "ValueRedeemed", "gwei")
	census.mTicketRedemptionError = stats.Int64("ticket_redemption_errors", "TicketRedemptionError", "tot")
	census.mSuggestedGasPrice = stats.Float64("suggested_gas_price", "SuggestedGasPrice", "gwei")
	census.mMinGasPrice = stats.Float64("min_gas_price", "MinGasPrice", "gwei")
	census.mMaxGasPrice = stats.Float64("max_gas_price", "MaxGasPrice", "gwei")
	census.mTranscodingPrice = stats.Float64("transcoding_price", "TranscodingPrice", "wei")

	// Metrics for pixel accounting
	census.mMilPixelsProcessed = stats.Float64("mil_pixels_processed", "MilPixelsProcessed", "mil pixels")

	// Metrics for fast verification
	census.mFastVerificationDone = stats.Int64("fast_verification_done", "FastVerificationDone", "tot")
	census.mFastVerificationFailed = stats.Int64("fast_verification_failed", "FastVerificationFailed", "tot")
	census.mFastVerificationEnabledCurrentSessions = stats.Int64("fast_verification_enabled_current_sessions_total",
		"Number of currently transcoded streams that have fast verification enabled", "tot")
	census.mFastVerificationUsingCurrentSessions = stats.Int64("fast_verification_using_current_sessions_total",
		"Number of currently transcoded streams that have fast verification enabled and that are using an untrusted orchestrator", "tot")

	glog.Infof("Compiler: %s Arch %s OS %s Go version %s", runtime.Compiler, runtime.GOARCH, runtime.GOOS, runtime.Version())
	glog.Infof("Livepeer version: %s", version)
	glog.Infof("Node type %s node ID %s", nodeType, NodeID)
	mVersions := stats.Int64("versions", "Version information.", "Num")
	compiler := tag.MustNewKey("compiler")
	goarch := tag.MustNewKey("goarch")
	goos := tag.MustNewKey("goos")
	goversion := tag.MustNewKey("goversion")
	livepeerversion := tag.MustNewKey("livepeerversion")
	ctx, err = tag.New(ctx, tag.Insert(census.kNodeType, string(nodeType)), tag.Insert(census.kNodeID, NodeID),
		tag.Insert(compiler, runtime.Compiler), tag.Insert(goarch, runtime.GOARCH), tag.Insert(goos, runtime.GOOS),
		tag.Insert(goversion, runtime.Version()), tag.Insert(livepeerversion, version))
	if err != nil {
		glog.Fatal("Error creating tagged context", err)
	}
	baseTags := []tag.Key{census.kNodeID, census.kNodeType}
	views := []*view.View{
		{
			Name:        "versions",
			Measure:     mVersions,
			Description: "Versions used by LivePeer node.",
			TagKeys:     []tag.Key{census.kNodeType, compiler, goos, goversion, livepeerversion},
			Aggregation: view.LastValue(),
		},
		{
			Name:        "broadcast_client_start_failed_total",
			Measure:     census.mStartBroadcastClientFailed,
			Description: "StartBroadcastClientFailed",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "stream_created_total",
			Measure:     census.mStreamCreated,
			Description: "StreamCreated",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "stream_started_total",
			Measure:     census.mStreamStarted,
			Description: "StreamStarted",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "stream_ended_total",
			Measure:     census.mStreamEnded,
			Description: "StreamEnded",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "stream_create_failed_total",
			Measure:     census.mStreamCreateFailed,
			Description: "StreamCreateFailed",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_timeout_1",
			Measure:     census.mHTTPClientTimeout1,
			Description: "Number of times HTTP connection was dropped before transcoding complete",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_timeout_2",
			Measure:     census.mHTTPClientTimeout2,
			Description: "Number of times HTTP connection was dropped before transcoded segments was sent back to client",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_segment_transcoded_realtime_3x",
			Measure:     census.mRealtime3x,
			Description: "Number of segment transcoded 3x faster than realtime",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_segment_transcoded_realtime_2x",
			Measure:     census.mRealtime2x,
			Description: "Number of segment transcoded 2x faster than realtime",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_segment_transcoded_realtime_1x",
			Measure:     census.mRealtime1x,
			Description: "Number of segment transcoded 1x faster than realtime",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_segment_transcoded_realtime_half",
			Measure:     census.mRealtimeHalf,
			Description: "Number of segment transcoded no more than two times slower than realtime",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "http_client_segment_transcoded_realtime_slow",
			Measure:     census.mRealtimeSlow,
			Description: "Number of segment transcoded more than two times slower than realtime",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_source_appeared_total",
			Measure:     census.mSegmentSourceAppeared,
			Description: "SegmentSourceAppeared",
			TagKeys:     append([]tag.Key{census.kProfile, census.kSegmentType}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_source_emerged_total",
			Measure:     census.mSegmentEmerged,
			Description: "SegmentEmerged",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_source_emerged_unprocessed_total",
			Measure:     census.mSegmentEmergedUnprocessed,
			Description: "Raw number of segments emerged from segmenter.",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_source_uploaded_total",
			Measure:     census.mSegmentUploaded,
			Description: "SegmentUploaded",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_source_upload_failed_total",
			Measure:     census.mSegmentUploadFailed,
			Description: "SegmentUploadedFailed",
			TagKeys:     append([]tag.Key{census.kErrorCode}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_transcoded_downloaded_total",
			Measure:     census.mSegmentDownloaded,
			Description: "SegmentDownloaded",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_transcoded_total",
			Measure:     census.mSegmentTranscoded,
			Description: "SegmentTranscoded",
			TagKeys:     append([]tag.Key{census.kProfiles, census.kTrusted, census.kVerified}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_transcoded_unprocessed_total",
			Measure:     census.mSegmentTranscodedUnprocessed,
			Description: "Raw number of segments successfully transcoded.",
			TagKeys:     append([]tag.Key{census.kProfiles}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_transcode_failed_total",
			Measure:     census.mSegmentTranscodeFailed,
			Description: "SegmentTranscodeFailed",
			TagKeys:     append([]tag.Key{census.kErrorCode}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_transcoded_appeared_total",
			Measure:     census.mSegmentTranscodedAppeared,
			Description: "SegmentTranscodedAppeared",
			TagKeys:     append([]tag.Key{census.kProfile, census.kSegmentType}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "segment_transcoded_all_appeared_total",
			Measure:     census.mSegmentTranscodedAllAppeared,
			Description: "SegmentTranscodedAllAppeared",
			TagKeys:     append([]tag.Key{census.kProfiles}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "success_rate",
			Measure:     census.mSuccessRate,
			Description: "Number of transcoded segments divided on number of source segments",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "transcode_time_seconds",
			Measure:     census.mTranscodeTime,
			Description: "TranscodeTime, seconds",
			TagKeys:     append([]tag.Key{census.kProfiles, census.kTrusted, census.kVerified}, baseTags...),
			Aggregation: view.Distribution(0, .250, .500, .750, 1.000, 1.250, 1.500, 2.000, 2.500, 3.000, 3.500, 4.000, 4.500, 5.000, 10.000),
		},
		{
			Name:        "transcode_latency_seconds",
			Measure:     census.mTranscodeLatency,
			Description: "Transcoding latency, from source segment emerged from segmenter till transcoded segment apeeared in manifest",
			TagKeys:     append([]tag.Key{census.kProfile}, baseTags...),
			Aggregation: view.Distribution(0, .500, .75, 1.000, 1.500, 2.000, 2.500, 3.000, 3.500, 4.000, 4.500, 5.000, 10.000),
		},
		{
			Name:        "transcode_overall_latency_seconds",
			Measure:     census.mTranscodeOverallLatency,
			Description: "Transcoding latency, from source segment emerged from segmenter till all transcoded segment apeeared in manifest",
			TagKeys:     append([]tag.Key{census.kProfiles}, baseTags...),
			Aggregation: view.Distribution(0, .500, .75, 1.000, 1.500, 2.000, 2.500, 3.000, 3.500, 4.000, 4.500, 5.000, 10.000),
		},
		{
			Name:        "transcode_score",
			Measure:     census.mTranscodeScore,
			Description: "Ratio of source segment duration vs. transcode time",
			TagKeys:     append([]tag.Key{census.kProfiles, census.kTrusted, census.kVerified}, baseTags...),
			Aggregation: view.Distribution(0, .5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, 10, 15, 20, 40),
		},
		{
			Name:        "recording_save_latency",
			Measure:     census.mRecordingSaveLatency,
			Description: "How long it takes to save segment to the OS",
			TagKeys:     baseTags,
			Aggregation: view.Distribution(0, .500, .75, 1.000, 1.500, 2.000, 2.500, 3.000, 3.500, 4.000, 4.500, 5.000, 10.000, 30.000),
		},
		{
			Name:        "recording_save_errors",
			Measure:     census.mRecordingSaveErrors,
			Description: "Number of errors during save to the recording OS",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "recording_saved_segments",
			Measure:     census.mRecordingSavedSegments,
			Description: "Number of segments saved to the recording OS",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "upload_time_seconds",
			Measure:     census.mUploadTime,
			Description: "UploadTime, seconds",
			TagKeys:     baseTags,
			Aggregation: view.Distribution(0, .10, .20, .50, .100, .150, .200, .500, .1000, .5000, 10.000),
		},
		{
			Name:        "download_time_seconds",
			Measure:     census.mDownloadTime,
			Description: "Download time",
			TagKeys:     baseTags,
			Aggregation: view.Distribution(0, .10, .20, .50, .100, .150, .200, .500, .1000, .5000, 10.000),
		},
		{
			Name:        "auth_webhook_time_milliseconds",
			Measure:     census.mAuthWebhookTime,
			Description: "Authentication webhook execution time, milliseconds",
			TagKeys:     baseTags,
			Aggregation: view.Distribution(0, 100, 250, 500, 750, 1000, 1500, 2000, 2500, 3000, 5000, 10000),
		},
		{
			Name:        "source_segment_duration_seconds",
			Measure:     census.mSourceSegmentDuration,
			Description: "Source segment's duration",
			TagKeys:     baseTags,
			Aggregation: view.Distribution(0, .5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5, 10, 15, 20),
		},
		{
			Name:        "max_sessions_total",
			Measure:     census.mMaxSessions,
			Description: "Max Sessions",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "current_sessions_total",
			Measure:     census.mCurrentSessions,
			Description: "Number of streams currently transcoding",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "discovery_errors_total",
			Measure:     census.mDiscoveryError,
			Description: "Number of discover errors",
			TagKeys:     append([]tag.Key{census.kErrorCode}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "transcode_retried",
			Measure:     census.mTranscodeRetried,
			Description: "Number of times segment transcode was retried",
			TagKeys:     append([]tag.Key{census.kTry}, baseTags...),
			Aggregation: view.Count(),
		},
		{
			Name:        "transcoders_number",
			Measure:     census.mTranscodersNumber,
			Description: "Number of transcoders currently connected to orchestrator",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "transcoders_capacity",
			Measure:     census.mTranscodersCapacity,
			Description: "Total advertised capacity of transcoders currently connected to orchestrator",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "transcoders_load",
			Measure:     census.mTranscodersLoad,
			Description: "Total load of transcoders currently connected to orchestrator",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "orchestrator_swaps",
			Measure:     census.mOrchestratorSwaps,
			Description: "Number of orchestrator swaps mid-stream",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},

		// Metrics for sending payments
		{
			Name:        "ticket_value_sent",
			Measure:     census.mTicketValueSent,
			Description: "Ticket value sent",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "tickets_sent",
			Measure:     census.mTicketsSent,
			Description: "Tickets sent",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "payment_create_errors",
			Measure:     census.mPaymentCreateError,
			Description: "Errors when creating payments",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "broadcaster_deposit",
			Measure:     census.mDeposit,
			Description: "Current remaining deposit for the broadcaster node",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "broadcaster_reserve",
			Measure:     census.mReserve,
			Description: "Current remaining reserve for the broadcaster node",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "max_transcoding_price",
			Measure:     census.mMaxTranscodingPrice,
			Description: "Maximum price per pixel to pay for transcoding",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},

		// Metrics for receiving payments
		{
			Name:        "ticket_value_recv",
			Measure:     census.mTicketValueRecv,
			Description: "Ticket value received",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "tickets_recv",
			Measure:     census.mTicketsRecv,
			Description: "Tickets received",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "payment_recv_errors",
			Measure:     census.mPaymentRecvErr,
			Description: "Errors when receiving payments",
			TagKeys:     append([]tag.Key{census.kErrorCode}, baseTags...),
			Aggregation: view.Sum(),
		},
		{
			Name:        "winning_tickets_recv",
			Measure:     census.mWinningTicketsRecv,
			Description: "Winning tickets received",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "value_redeemed",
			Measure:     census.mValueRedeemed,
			Description: "Winning ticket value redeemed",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "ticket_redemption_errors",
			Measure:     census.mTicketRedemptionError,
			Description: "Errors when redeeming tickets",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "min_gas_price",
			Measure:     census.mMinGasPrice,
			Description: "Minimum gas price to use for gas price suggestions",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "max_gas_price",
			Measure:     census.mMaxGasPrice,
			Description: "Maximum gas price to use for gas price suggestions",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},

		// Metrics for pixel accounting
		{
			Name:        "mil_pixels_processed",
			Measure:     census.mMilPixelsProcessed,
			Description: "Million pixels processed",
			TagKeys:     baseTags,
			Aggregation: view.Sum(),
		},
		{
			Name:        "suggested_gas_price",
			Measure:     census.mSuggestedGasPrice,
			Description: "Suggested gas price for winning ticket redemption",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "transcoding_price",
			Measure:     census.mTranscodingPrice,
			Description: "Transcoding price per pixel",
			TagKeys:     append([]tag.Key{census.kSender}, baseTags...),
			Aggregation: view.LastValue(),
		},

		// Metrics for fast verification
		{
			Name:        "fast_verification_done",
			Measure:     census.mFastVerificationDone,
			Description: "Number of fast verifications done",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "fast_verification_failed",
			Measure:     census.mFastVerificationFailed,
			Description: "Number of fast verifications failed",
			TagKeys:     baseTags,
			Aggregation: view.Count(),
		},
		{
			Name:        "fast_verification_enabled_current_sessions_total",
			Measure:     census.mFastVerificationEnabledCurrentSessions,
			Description: "Number of currently transcoded streams that have fast verification enabled",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
		{
			Name:        "fast_verification_using_current_sessions_total",
			Measure:     census.mFastVerificationUsingCurrentSessions,
			Description: "Number of currently transcoded streams that have fast verification enabled and that are using an untrusted orchestrator",
			TagKeys:     baseTags,
			Aggregation: view.LastValue(),
		},
	}

	// Register the views
	if err := view.Register(views...); err != nil {
		glog.Fatalf("Failed to register views: %v", err)
	}
	registry := rprom.NewRegistry()
	registry.MustRegister(rprom.NewProcessCollector(rprom.ProcessCollectorOpts{}))
	registry.MustRegister(rprom.NewGoCollector())
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "livepeer",
		Registry:  registry,
	})
	if err != nil {
		glog.Fatalf("Failed to create the Prometheus stats exporter: %v", err)
	}

	// Register the Prometheus exporters as a stats exporter.
	view.RegisterExporter(pe)
	stats.Record(ctx, mVersions.M(1))
	ctx, err = tag.New(census.ctx, tag.Insert(census.kErrorCode, "LostSegment"))
	if err != nil {
		glog.Fatal("Error creating context", err)
	}
	if !unitTestMode {
		go census.timeoutWatcher(ctx)
	}
	Exporter = pe

	// init metrics values
	SetTranscodersNumberAndLoad(0, 0, 0)
}

// LogDiscoveryError records discovery error
func LogDiscoveryError(code string) {
	if strings.Contains(code, "OrchestratorCapped") {
		code = "OrchestratorCapped"
	} else if strings.Contains(code, "Canceled") {
		code = "Canceled"
	}
	ctx, err := tag.New(census.ctx, tag.Insert(census.kErrorCode, code))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	stats.Record(ctx, census.mDiscoveryError.M(1))
}

func (cen *censusMetricsCounter) successRate() float64 {
	var i int
	var f float64
	if len(cen.success) == 0 {
		return 1
	}
	for _, avg := range cen.success {
		if r, has := avg.successRate(); has {
			i++
			f += r
		}
	}
	if i > 0 {
		return f / float64(i)
	}
	return 1
}

func (sa *segmentsAverager) successRate() (float64, bool) {
	var emerged, transcoded int
	if sa.end == -1 {
		return 1, false
	}
	i := sa.start
	now := time.Now()
	for {
		item := &sa.segments[i]
		if item.transcoded > 0 || item.failed || now.Sub(item.emergedTime) > timeToWaitForError {
			emerged += item.emerged
			transcoded += item.transcoded
		}
		if i == sa.end {
			break
		}
		i = sa.advance(i)
	}
	if emerged > 0 {
		return float64(transcoded) / float64(emerged), true
	}
	return 1, false
}

func (sa *segmentsAverager) advance(i int) int {
	i++
	if i == len(sa.segments) {
		i = 0
	}
	return i
}

func (sa *segmentsAverager) addEmerged(seqNo uint64) {
	item, _ := sa.getAddItem(seqNo)
	item.emerged = 1
	item.transcoded = 0
	item.emergedTime = time.Now()
	item.seqNo = seqNo
}

func (sa *segmentsAverager) addTranscoded(seqNo uint64, failed bool) {
	item, found := sa.getAddItem(seqNo)
	if !found {
		item.emerged = 0
		item.emergedTime = time.Now()
	}
	item.failed = failed
	if !failed {
		item.transcoded = 1
	}
	item.seqNo = seqNo
}

func (sa *segmentsAverager) getAddItem(seqNo uint64) (*segmentCount, bool) {
	var index int
	if sa.end == -1 {
		sa.end = 0
	} else {
		i := sa.start
		for {
			if sa.segments[i].seqNo == seqNo {
				return &sa.segments[i], true
			}
			if i == sa.end {
				break
			}
			i = sa.advance(i)
		}
		sa.end = sa.advance(sa.end)
		index = sa.end
		if sa.end == sa.start {
			sa.start = sa.advance(sa.start)
		}
	}
	return &sa.segments[index], false
}

func (sa *segmentsAverager) canBeRemoved() bool {
	if sa.end == -1 {
		return true
	}
	i := sa.start
	now := time.Now()
	for {
		item := &sa.segments[i]
		if item.transcoded == 0 && !item.failed && now.Sub(item.emergedTime) <= timeToWaitForError {
			return false
		}
		if i == sa.end {
			break
		}
		i = sa.advance(i)
	}
	return true
}

func (cen *censusMetricsCounter) timeoutWatcher(ctx context.Context) {
	for {
		cen.lock.Lock()
		now := time.Now()
		for nonce, emerged := range cen.emergeTimes {
			for seqNo, tm := range emerged {
				ago := now.Sub(tm)
				if ago > timeToWaitForError {
					stats.Record(cen.ctx, cen.mSegmentEmerged.M(1))
					delete(emerged, seqNo)
					// This shouldn't happen, but if it is, we record
					// `LostSegment` error, to try to find out why we missed segment
					stats.Record(ctx, cen.mSegmentTranscodeFailed.M(1))
					glog.Errorf("LostSegment nonce=%d seqNo=%d emerged=%ss ago", nonce, seqNo, ago)
				}
			}
		}
		cen.sendSuccess()
		for nonce, avg := range cen.success {
			if avg.removed && now.Sub(avg.removedAt) > 2*timeToWaitForError {
				// need to keep this around for some time to give Prometheus chance to scrape this value
				// (Prometheus scrapes every 5 seconds)
				delete(cen.success, nonce)
			} else {
				for seqNo, tr := range avg.tries {
					if now.Sub(tr.first) > 2*timeToWaitForError {
						delete(avg.tries, seqNo)
					}
				}
			}
		}
		cen.lock.Unlock()
		time.Sleep(timeoutWatcherPause)
	}
}

func MaxSessions(maxSessions int) {
	census.lock.Lock()
	defer census.lock.Unlock()
	stats.Record(census.ctx, census.mMaxSessions.M(int64(maxSessions)))
}

func OrchestratorSwapped() {
	census.lock.Lock()
	defer census.lock.Unlock()
	stats.Record(census.ctx, census.mOrchestratorSwaps.M(1))
}

func CurrentSessions(currentSessions int) {
	census.lock.Lock()
	defer census.lock.Unlock()
	stats.Record(census.ctx, census.mCurrentSessions.M(int64(currentSessions)))
}

func FastVerificationEnabledAndUsingCurrentSessions(enabled, using int) {
	stats.Record(census.ctx, census.mFastVerificationEnabledCurrentSessions.M(int64(enabled)), census.mFastVerificationUsingCurrentSessions.M(int64(using)))
}

func TranscodeTry(nonce, seqNo uint64) {
	census.lock.Lock()
	defer census.lock.Unlock()
	if av, ok := census.success[nonce]; ok {
		if av.tries == nil {
			av.tries = make(map[uint64]tryData)
		}
		try := 1
		if ts, tok := av.tries[seqNo]; tok {
			ts.tries++
			try = ts.tries
			av.tries[seqNo] = ts
			label := ">10"
			if ts.tries < 11 {
				label = strconv.Itoa(ts.tries)
			}
			ctx, err := tag.New(census.ctx, tag.Insert(census.kTry, label))
			if err != nil {
				glog.Error("Error creating context", err)
				return
			}
			stats.Record(ctx, census.mTranscodeRetried.M(1))
		} else {
			av.tries[seqNo] = tryData{tries: 1, first: time.Now()}
		}
		glog.V(logLevel).Infof("Trying to transcode segment nonce=%d seqNo=%d try=%d", nonce, seqNo, try)
	}
}

func SetTranscodersNumberAndLoad(load, capacity, number int) {
	census.lock.Lock()
	defer census.lock.Unlock()
	stats.Record(census.ctx, census.mTranscodersLoad.M(int64(load)))
	stats.Record(census.ctx, census.mTranscodersCapacity.M(int64(capacity)))
	stats.Record(census.ctx, census.mTranscodersNumber.M(int64(number)))
}

func SegmentEmerged(nonce, seqNo uint64, profilesNum int, dur float64) {
	glog.V(logLevel).Infof("Logging SegmentEmerged... nonce=%d seqNo=%d duration=%v", nonce, seqNo, dur)
	if census.nodeType == Broadcaster {
		census.segmentEmerged(nonce, seqNo, profilesNum)
	}
	stats.Record(census.ctx, census.mSourceSegmentDuration.M(dur))
}

func (cen *censusMetricsCounter) segmentEmerged(nonce, seqNo uint64, profilesNum int) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	if _, has := cen.emergeTimes[nonce]; !has {
		cen.emergeTimes[nonce] = make(map[uint64]time.Time)
	}
	if avg, has := cen.success[nonce]; has {
		avg.addEmerged(seqNo)
	}
	cen.emergeTimes[nonce][seqNo] = time.Now()
	stats.Record(cen.ctx, cen.mSegmentEmergedUnprocessed.M(1))
}

func SourceSegmentAppeared(nonce, seqNo uint64, manifestID, profile string, recordingEnabled bool) {
	glog.V(logLevel).Infof("Logging SourceSegmentAppeared... nonce=%d manifestID=%s seqNo=%d profile=%s", nonce,
		manifestID, seqNo, profile)
	census.segmentSourceAppeared(nonce, seqNo, profile, recordingEnabled)
}

func (cen *censusMetricsCounter) segmentSourceAppeared(nonce, seqNo uint64, profile string, recordingEnabled bool) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	ctx, err := tag.New(cen.ctx, tag.Insert(census.kProfile, profile))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	segType := segTypeRegular
	if recordingEnabled {
		segType = segTypeRec
	}
	ctx, err = tag.New(ctx, tag.Insert(cen.kSegmentType, segType))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	stats.Record(ctx, cen.mSegmentSourceAppeared.M(1))
}

func SegmentUploaded(nonce, seqNo uint64, uploadDur time.Duration) {
	glog.V(logLevel).Infof("Logging SegmentUploaded... nonce=%d seqNo=%d dur=%s", nonce, seqNo, uploadDur)
	census.segmentUploaded(nonce, seqNo, uploadDur)
}

func (cen *censusMetricsCounter) segmentUploaded(nonce, seqNo uint64, uploadDur time.Duration) {
	stats.Record(cen.ctx, cen.mSegmentUploaded.M(1), cen.mUploadTime.M(uploadDur.Seconds()))
}

func SegmentDownloaded(nonce, seqNo uint64, downloadDur time.Duration) {
	glog.V(logLevel).Infof("Logging SegmentDownloaded... nonce=%d seqNo=%d dur=%s", nonce, seqNo, downloadDur)
	census.segmentDownloaded(nonce, seqNo, downloadDur)
}

func (cen *censusMetricsCounter) segmentDownloaded(nonce, seqNo uint64, downloadDur time.Duration) {
	stats.Record(cen.ctx, cen.mSegmentDownloaded.M(1), cen.mDownloadTime.M(downloadDur.Seconds()))
}

func HTTPClientTimedOut1() {
	stats.Record(census.ctx, census.mHTTPClientTimeout1.M(1))
}

func HTTPClientTimedOut2() {
	stats.Record(census.ctx, census.mHTTPClientTimeout2.M(1))
}

func SegmentFullyProcessed(segDur, processDur float64) {
	if processDur == 0 {
		return
	}
	xRealtime := processDur / segDur
	switch {
	case xRealtime < 1.0/3.0:
		stats.Record(census.ctx, census.mRealtime3x.M(1))
	case xRealtime < 1.0/2.0:
		stats.Record(census.ctx, census.mRealtime2x.M(1))
	case xRealtime < 1.0:
		stats.Record(census.ctx, census.mRealtime1x.M(1))
	case xRealtime < 2.0:
		stats.Record(census.ctx, census.mRealtimeHalf.M(1))
	default:
		stats.Record(census.ctx, census.mRealtimeSlow.M(1))
	}
}

func AuthWebhookFinished(dur time.Duration) {
	census.authWebhookFinished(dur)
}

func (cen *censusMetricsCounter) authWebhookFinished(dur time.Duration) {
	stats.Record(cen.ctx, cen.mAuthWebhookTime.M(float64(dur)/float64(time.Millisecond)))
}

func SegmentUploadFailed(nonce, seqNo uint64, code SegmentUploadError, err error, permanent bool) {
	if code == SegmentUploadErrorUnknown {
		reason := err.Error()
		var timedout bool
		if err, ok := err.(net.Error); ok && err.Timeout() {
			timedout = true
		}
		if timedout || strings.Contains(reason, "Client.Timeout") || strings.Contains(reason, "timed out") ||
			strings.Contains(reason, "timeout") || strings.Contains(reason, "context deadline exceeded") ||
			strings.Contains(reason, "EOF") {
			code = SegmentUploadErrorTimeout
		} else if reason == "Session ended" {
			code = SegmentUploadErrorSessionEnded
		}
	}
	glog.Errorf("Logging SegmentUploadFailed... code=%v reason='%s'", code, err.Error())

	census.segmentUploadFailed(nonce, seqNo, code, permanent)
}

func (cen *censusMetricsCounter) segmentUploadFailed(nonce, seqNo uint64, code SegmentUploadError, permanent bool) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	if permanent {
		cen.countSegmentEmerged(nonce, seqNo)
	}

	ctx, err := tag.New(cen.ctx, tag.Insert(census.kErrorCode, string(code)))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	stats.Record(ctx, cen.mSegmentUploadFailed.M(1))
	if permanent {
		cen.countSegmentTranscoded(nonce, seqNo, true)
		cen.sendSuccess()
	}
}

func SegmentTranscoded(nonce, seqNo uint64, sourceDur time.Duration, transcodeDur time.Duration, profiles string,
	trusted, verified bool) {

	glog.V(logLevel).Infof("Logging SegmentTranscode nonce=%d seqNo=%d dur=%s trusted=%v verified=%v", nonce, seqNo, transcodeDur, trusted, verified)
	census.segmentTranscoded(nonce, seqNo, sourceDur, transcodeDur, profiles, trusted, verified)
}

func (cen *censusMetricsCounter) segmentTranscoded(nonce, seqNo uint64, sourceDur time.Duration, transcodeDur time.Duration,
	profiles string, trusted, verified bool) {

	cen.lock.Lock()
	defer cen.lock.Unlock()
	verifiedStr := "verified"
	if !verified {
		verifiedStr = "unverified"
	}
	trustedStr := "trusted"
	if !trusted {
		trustedStr = "untrusted"
	}
	ctx, err := tag.New(cen.ctx, tag.Insert(cen.kProfiles, profiles), tag.Insert(cen.kVerified, verifiedStr), tag.Insert(cen.kTrusted, trustedStr))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	stats.Record(ctx, cen.mSegmentTranscoded.M(1), cen.mTranscodeTime.M(transcodeDur.Seconds()), cen.mTranscodeScore.M(sourceDur.Seconds()/transcodeDur.Seconds()))
}

func SegmentTranscodeFailed(subType SegmentTranscodeError, nonce, seqNo uint64, err error, permanent bool) {
	glog.Errorf("Logging SegmentTranscodeFailed subtype=%v nonce=%d seqNo=%d err=%q", subType, nonce, seqNo, err.Error())
	census.segmentTranscodeFailed(nonce, seqNo, subType, permanent)
}

func (cen *censusMetricsCounter) segmentTranscodeFailed(nonce, seqNo uint64, code SegmentTranscodeError, permanent bool) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	ctx, err := tag.New(cen.ctx, tag.Insert(census.kErrorCode, string(code)))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	stats.Record(ctx, cen.mSegmentTranscodeFailed.M(1))
	if permanent {
		cen.countSegmentEmerged(nonce, seqNo)
		cen.countSegmentTranscoded(nonce, seqNo, code != SegmentTranscodeErrorSessionEnded)
		cen.sendSuccess()
	}
}

func (cen *censusMetricsCounter) countSegmentTranscoded(nonce, seqNo uint64, failed bool) {
	if avg, ok := cen.success[nonce]; ok {
		avg.addTranscoded(seqNo, failed)
	}
}

func (cen *censusMetricsCounter) countSegmentEmerged(nonce, seqNo uint64) {
	if _, ok := cen.emergeTimes[nonce][seqNo]; ok {
		stats.Record(cen.ctx, cen.mSegmentEmerged.M(1))
		delete(cen.emergeTimes[nonce], seqNo)
	}
}

func (cen *censusMetricsCounter) sendSuccess() {
	stats.Record(cen.ctx, cen.mSuccessRate.M(cen.successRate()))
}

func SegmentFullyTranscoded(nonce, seqNo uint64, profiles string, errCode SegmentTranscodeError) {
	census.lock.Lock()
	defer census.lock.Unlock()
	ctx, err := tag.New(census.ctx, tag.Insert(census.kProfiles, profiles))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}

	if st, ok := census.emergeTimes[nonce][seqNo]; ok {
		if errCode == "" {
			latency := time.Since(st)
			stats.Record(ctx, census.mTranscodeOverallLatency.M(latency.Seconds()))
		}
		census.countSegmentEmerged(nonce, seqNo)
	}
	if errCode == "" {
		stats.Record(ctx, census.mSegmentTranscodedAllAppeared.M(1))
	}
	failed := errCode != "" && errCode != SegmentTranscodeErrorSessionEnded
	census.countSegmentTranscoded(nonce, seqNo, failed)
	if !failed {
		stats.Record(ctx, census.mSegmentTranscodedUnprocessed.M(1))
	}
	census.sendSuccess()
}

func RecordingPlaylistSaved(dur time.Duration, err error) {
	if err != nil {
		stats.Record(census.ctx, census.mRecordingSaveErrors.M(1))
	} else {
		stats.Record(census.ctx, census.mRecordingSaveLatency.M(dur.Seconds()))
	}
}

func RecordingSegmentSaved(dur time.Duration, err error) {
	if err != nil {
		stats.Record(census.ctx, census.mRecordingSaveErrors.M(1))
	} else {
		stats.Record(census.ctx, census.mRecordingSaveLatency.M(dur.Seconds()))
		stats.Record(census.ctx, census.mRecordingSavedSegments.M(1))
	}
}

func TranscodedSegmentAppeared(nonce, seqNo uint64, profile string, recordingEnabled bool) {
	glog.V(logLevel).Infof("Logging LogTranscodedSegmentAppeared... nonce=%d seqNo=%d profile=%s", nonce, seqNo, profile)
	census.segmentTranscodedAppeared(nonce, seqNo, profile, recordingEnabled)
}

func (cen *censusMetricsCounter) segmentTranscodedAppeared(nonce, seqNo uint64, profile string, recordingEnabled bool) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	ctx, err := tag.New(cen.ctx, tag.Insert(cen.kProfile, profile))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}

	// cen.transcodedSegments[nonce] = cen.transcodedSegments[nonce] + 1
	if st, ok := cen.emergeTimes[nonce][seqNo]; ok {
		latency := time.Since(st)
		glog.V(logLevel).Infof("Recording latency for segment nonce=%d seqNo=%d profile=%s latency=%s", nonce, seqNo, profile, latency)
		stats.Record(ctx, cen.mTranscodeLatency.M(latency.Seconds()))
	}

	segType := segTypeRegular
	if recordingEnabled {
		segType = segTypeRec
	}
	ctx, err = tag.New(ctx, tag.Insert(cen.kSegmentType, segType))
	if err != nil {
		glog.Error("Error creating context", err)
		return
	}
	stats.Record(ctx, cen.mSegmentTranscodedAppeared.M(1))
}

func StreamCreateFailed(nonce uint64, reason string) {
	glog.Errorf("Logging StreamCreateFailed... nonce=%d reason='%s'", nonce, reason)
	census.streamCreateFailed(nonce, reason)
}

func (cen *censusMetricsCounter) streamCreateFailed(nonce uint64, reason string) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	stats.Record(cen.ctx, cen.mStreamCreateFailed.M(1))
}

func newAverager() *segmentsAverager {
	return &segmentsAverager{
		segments: make([]segmentCount, numberOfSegmentsToCalcAverage),
		end:      -1,
	}
}

func StreamCreated(hlsStrmID string, nonce uint64) {
	glog.V(logLevel).Infof("Logging StreamCreated... nonce=%d strid=%s", nonce, hlsStrmID)
	census.streamCreated(nonce)
}

func (cen *censusMetricsCounter) streamCreated(nonce uint64) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	stats.Record(cen.ctx, cen.mStreamCreated.M(1))
	cen.success[nonce] = newAverager()
}

func StreamStarted(nonce uint64) {
	glog.V(logLevel).Infof("Logging StreamStarted... nonce=%d", nonce)
	census.streamStarted(nonce)
}

func (cen *censusMetricsCounter) streamStarted(nonce uint64) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	stats.Record(cen.ctx, cen.mStreamStarted.M(1))
}

func StreamEnded(nonce uint64) {
	glog.V(logLevel).Infof("Logging StreamEnded... nonce=%d", nonce)
	census.streamEnded(nonce)
}

func (cen *censusMetricsCounter) streamEnded(nonce uint64) {
	cen.lock.Lock()
	defer cen.lock.Unlock()
	stats.Record(cen.ctx, cen.mStreamEnded.M(1))
	delete(cen.emergeTimes, nonce)
	if avg, has := cen.success[nonce]; has {
		if avg.canBeRemoved() {
			delete(cen.success, nonce)
		} else {
			avg.removed = true
			avg.removedAt = time.Now()
		}
	}
	census.sendSuccess()
}

// TicketValueSent records the ticket value sent
func TicketValueSent(value *big.Rat) {
	census.lock.Lock()
	defer census.lock.Unlock()

	if value.Cmp(big.NewRat(0, 1)) <= 0 {
		return
	}

	stats.Record(census.ctx, census.mTicketValueSent.M(fracwei2gwei(value)))
}

// TicketsSent records the number of tickets sent
func TicketsSent(numTickets int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	if numTickets <= 0 {
		return
	}

	stats.Record(census.ctx, census.mTicketsSent.M(int64(numTickets)))
}

// PaymentCreateError records a error from payment creation
func PaymentCreateError() {
	census.lock.Lock()
	defer census.lock.Unlock()

	stats.Record(census.ctx, census.mPaymentCreateError.M(1))
}

// Deposit records the current deposit for the broadcaster
func Deposit(sender string, deposit *big.Int) {
	stats.Record(census.ctx, census.mDeposit.M(wei2gwei(deposit)))
}

func Reserve(sender string, reserve *big.Int) {
	stats.Record(census.ctx, census.mReserve.M(wei2gwei(reserve)))
}

func MaxTranscodingPrice(maxPrice *big.Rat) {
	census.lock.Lock()
	defer census.lock.Unlock()

	floatWei, ok := maxPrice.Float64()
	if ok {
		stats.Record(census.ctx, census.mTranscodingPrice.M(floatWei))
	}
}

// TicketValueRecv records the ticket value received from a sender for a manifestID
func TicketValueRecv(value *big.Rat) {
	census.lock.Lock()
	defer census.lock.Unlock()

	if value.Cmp(big.NewRat(0, 1)) <= 0 {
		return
	}

	stats.Record(census.ctx, census.mTicketValueRecv.M(fracwei2gwei(value)))
}

// TicketsRecv records the number of tickets received from a sender for a manifestID
func TicketsRecv(numTickets int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	if numTickets <= 0 {
		return
	}

	stats.Record(census.ctx, census.mTicketsRecv.M(int64(numTickets)))
}

// PaymentRecvError records an error from receiving a payment
func PaymentRecvError(errStr string) {
	census.lock.Lock()
	defer census.lock.Unlock()

	var errCode string
	if strings.Contains(errStr, "Expected price") {
		errCode = "InvalidPrice"
	} else if strings.Contains(errStr, "invalid already revealed recipientRand") {
		errCode = "InvalidRecipientRand"
	} else if strings.Contains(errStr, "invalid ticket faceValue") {
		errCode = "InvalidTicketFaceValue"
	} else if strings.Contains(errStr, "invalid ticket winProb") {
		errCode = "InvalidTicketWinProb"
	} else {
		errCode = "PaymentError"
	}

	ctx, err := tag.New(
		census.ctx,
		tag.Insert(census.kErrorCode, errCode),
	)
	if err != nil {
		glog.Fatal(err)
	}

	stats.Record(ctx, census.mPaymentRecvErr.M(1))
}

// WinningTicketsRecv records the number of winning tickets received
func WinningTicketsRecv(numTickets int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	if numTickets <= 0 {
		return
	}

	stats.Record(census.ctx, census.mWinningTicketsRecv.M(int64(numTickets)))
}

// ValueRedeemed records the value from redeeming winning tickets
func ValueRedeemed(value *big.Int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	if value.Cmp(big.NewInt(0)) <= 0 {
		return
	}

	stats.Record(census.ctx, census.mValueRedeemed.M(wei2gwei(value)))
}

// TicketRedemptionError records an error from redeeming a ticket
func TicketRedemptionError() {
	census.lock.Lock()
	defer census.lock.Unlock()

	stats.Record(census.ctx, census.mTicketRedemptionError.M(1))
}

func MilPixelsProcessed(milPixels float64) {
	census.lock.Lock()
	defer census.lock.Unlock()

	stats.Record(census.ctx, census.mMilPixelsProcessed.M(milPixels))
}

// SuggestedGasPrice records the last suggested gas price
func SuggestedGasPrice(gasPrice *big.Int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	stats.Record(census.ctx, census.mSuggestedGasPrice.M(wei2gwei(gasPrice)))
}

func MinGasPrice(minGasPrice *big.Int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	stats.Record(census.ctx, census.mMinGasPrice.M(wei2gwei(minGasPrice)))
}

func MaxGasPrice(maxGasPrice *big.Int) {
	census.lock.Lock()
	defer census.lock.Unlock()

	stats.Record(census.ctx, census.mMaxGasPrice.M(wei2gwei(maxGasPrice)))
}

// TranscodingPrice records the last transcoding price
func TranscodingPrice(sender string, price *big.Rat) {
	census.lock.Lock()
	defer census.lock.Unlock()

	floatWei, ok := price.Float64()
	if ok {
		stats.Record(census.ctx, census.mTranscodingPrice.M(floatWei))
	}
}

// Convert wei to gwei
func wei2gwei(wei *big.Int) float64 {
	gwei, _ := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(float64(gweiConversionFactor))).Float64()
	return gwei
}

// Convert fractional wei to gwei
func fracwei2gwei(wei *big.Rat) float64 {
	floatWei, _ := wei.Float64()
	return floatWei / gweiConversionFactor
}

func FastVerificationDone() {
	stats.Record(census.ctx, census.mFastVerificationDone.M(1))
}

func FastVerificationFailed() {
	stats.Record(census.ctx, census.mFastVerificationFailed.M(1))
}
