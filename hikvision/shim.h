/*
 * shim.h - plain-C ABI surface between Go (cgo) and the Hikvision HCNetSDK.
 *
 * WHY THIS FILE EXISTS: HCNetSDK.h declares every NET_DVR_* function through a
 * macro (NET_DVR_API) that unconditionally expands to `extern "C" ...` on both
 * Windows and Linux - not guarded by `#ifdef __cplusplus`. That means the vendor
 * header is only valid inside a C++ translation unit; a plain C compiler (which
 * is what cgo uses for the preamble above `import "C"`) fails immediately with
 * "expected identifier or '(' before string constant" on every declaration.
 *
 * So shim.cpp (compiled as C++ via cgo's CXX support) is the only file that
 * includes HCNetSDK.h. It exposes the curated subset of the SDK used by this
 * package through the plain-C functions and POD structs declared here, using
 * only fixed-width types so the memory layout cannot drift between the
 * MSVC-flavoured Windows header and the gcc-flavoured Linux one. Go's cgo
 * preamble includes only this file.
 */
#ifndef HIKGO_SHIM_H
#define HIKGO_SHIM_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ============================== lifecycle ============================== */

int32_t hik_init(void);
int32_t hik_cleanup(void);
uint32_t hik_get_sdk_version(void);
uint32_t hik_get_sdk_build_version(void);

/* NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, ...) - tells the SDK where
 * its own component directory (HCNetSDKCom/, libcrypto, libssl, ...) lives, so
 * that no LD_LIBRARY_PATH/PATH manipulation is required at runtime. Must be
 * called before hik_init(). */
int32_t hik_set_sdk_component_path(const char *path);

int32_t hik_set_connect_time(uint32_t wait_time_ms, uint32_t try_times);
int32_t hik_set_reconnect(uint32_t interval_ms, int32_t enable);
int32_t hik_set_log_to_file(int32_t log_level, const char *log_dir, int32_t auto_delete);

/* ================================ errors ================================ */

uint32_t hik_get_last_error(void);
/* Returned pointer is owned by the SDK (static buffer) - copy it immediately,
 * do not free it. */
const char *hik_get_error_msg(void);

/* ============================ login / device ============================= */

typedef struct {
    char device_address[132];
    uint16_t port;
    char user_name[68];
    char password[68];
    int32_t use_https; /* 0-tcp, 1-tls, 2-adapt */
} hik_login_info;

typedef struct {
    uint8_t serial_number[48];
    uint8_t alarm_in_port_num;
    uint8_t alarm_out_port_num;
    uint8_t disk_num;
    uint8_t dvr_type;
    uint8_t chan_num;
    uint8_t start_chan;
    uint8_t ip_chan_num_low;
    uint8_t start_dchan;
    uint8_t high_dchan_num;
    uint32_t ip_chan_num; /* combined byIPChanNum + byHighDChanNum*256, best-effort channel count */
    uint16_t dev_type;
} hik_device_info;

/* Returns the user ID (>=0) on success, or a negative value on failure (call
 * hik_get_last_error() for the reason). */
int32_t hik_login(const hik_login_info *in, hik_device_info *out);
int32_t hik_logout(int32_t user_id);

/* ================================ preview ================================ */

/* Frame data is delivered to the Go-exported goRealDataTrampoline (see
 * callbacks.go) - shim.cpp calls it directly (declared `extern` in shim.cpp)
 * rather than Go passing a function pointer in: cgo cannot take the address
 * of one of its own //export functions as a C.xxx value from Go code, so the
 * shim always wires up its own fixed C++ trampoline and that trampoline
 * calls the fixed Go entry point by name. user is an opaque value (a
 * runtime/cgo.Handle bit pattern) round-tripped back to the trampoline
 * unchanged; the Go side must copy the buffer's contents before returning -
 * it is only valid for the duration of the call. */
int32_t hik_realplay_start(int32_t user_id, int32_t channel, uint32_t stream_type, uintptr_t user);
int32_t hik_realplay_stop(int32_t real_handle);

/* Registers a single process-wide exception/reconnect callback, delivered to
 * goExceptionTrampoline. */
int32_t hik_set_exception_callback(void);

/* ================================== PTZ ================================== */

int32_t hik_ptz_control(int32_t user_id, int32_t channel, uint32_t command,
                         uint32_t stop, uint32_t speed);
int32_t hik_ptz_preset(int32_t user_id, int32_t channel, uint32_t preset_cmd, uint32_t preset_index);

/* ================================ capture ================================ */

/* jpeg_quality: 0-best, 1-better, 2-average. pic_size: HCNetSDK resolution
 * code, 0xff = auto (use current stream resolution). buf/buf_cap is caller-
 * owned; returns the number of bytes written via *out_len. */
int32_t hik_capture_jpeg(int32_t user_id, int32_t channel, uint16_t pic_size,
                          uint16_t jpeg_quality, uint8_t *buf, uint32_t buf_cap, uint32_t *out_len);

/* ================================ playback ================================ */

/* year/month/day/hour/minute/second are the device's wall-clock reading -
 * NOT necessarily UTC. tz_valid/tz_offset_hour/tz_offset_min mirror the SDK's
 * cTimeDifferenceH/cTimeDifferenceM/byISO8601(or byLocalOrUTC) fields, which
 * (when tz_valid is non-zero) give the UTC offset that wall-clock reading is
 * in: UTC = wall-clock - (tz_offset_hour hours + tz_offset_min minutes).
 * tz_valid is 0 for hik_time values built on the Go side to send to the SDK
 * (start/stop query bounds), since those commands take device-local time
 * with no accompanying offset. */
typedef struct {
    uint16_t year;
    uint8_t month, day, hour, minute, second;
    int8_t tz_offset_hour;
    int8_t tz_offset_min;
    uint8_t tz_valid;
} hik_time;

int32_t hik_find_file_open(int32_t user_id, int32_t channel, hik_time start, hik_time stop);

typedef struct {
    char file_name[100];
    hik_time start_time;
    hik_time stop_time;
    uint32_t file_size;
    uint8_t locked;
    uint8_t file_type;
} hik_find_data;

/* Returns: 0=NET_DVR_ISFINDING(keep polling), 1000=NET_DVR_FILE_SUCCESS(data
 * filled), 1001=NET_DVR_FILE_NOFIND, 1002=NET_DVR_NOMOREFILE, <0=error. */
int32_t hik_find_file_next(int32_t find_handle, hik_find_data *out);
int32_t hik_find_file_close(int32_t find_handle);

/* Delivers to goPlaybackDataTrampoline - see the note on hik_realplay_start. */
int32_t hik_playback_start_by_time(int32_t user_id, int32_t channel, hik_time start, hik_time stop, uintptr_t user);
int32_t hik_playback_control(int32_t play_handle, uint32_t control_code);
int32_t hik_playback_stop(int32_t play_handle);

int32_t hik_download_start_by_time(int32_t user_id, int32_t channel, hik_time start, hik_time stop,
                                    const char *saved_file_name);
int32_t hik_download_get_progress(int32_t file_handle); /* 0-100, or <0 on error/done */
int32_t hik_download_stop(int32_t file_handle);

/* ================================= alarm ================================= */

typedef struct {
    uint8_t user_id_valid;
    int32_t user_id;
    uint8_t device_ip_valid;
    char device_ip[128];
    uint8_t serial_valid;
    uint8_t serial_number[48];
} hik_alarmer;

/* command constants (subset of HCNetSDK.h's COMM_* codes relevant to this
 * package - mirrored here as plain ints so Go doesn't need the vendor header). */
#define HIK_COMM_ALARM 0x1100
#define HIK_COMM_ALARM_V30 0x4000
#define HIK_COMM_UPLOAD_PLATE_RESULT 0x2800
#define HIK_COMM_ITS_PLATE_RESULT 0x3050
#define HIK_COMM_PLATE_RESULT_V50 0x3063

typedef struct {
    char license[32];       /* recognized plate text, UTF-8 */
    uint8_t confidence;      /* 0-100 */
    uint8_t plate_color;
    uint8_t vehicle_color;
    uint8_t vehicle_type;
    uint16_t speed_kmh;
    hik_time capture_time;
    /* scene/plate snapshot JPEGs - pointers are only valid for the duration of
     * the callback; Go must copy them immediately. May be NULL/0 if the event
     * carried no image of that kind. */
    const uint8_t *scene_image;
    uint32_t scene_image_len;
    const uint8_t *plate_image;
    uint32_t plate_image_len;
} hik_plate_event;

/* Registers the single, process-wide alarm message callback
 * (NET_DVR_SetDVRMessageCallBack_V51), delivered to goAlarmTrampoline. is_plate
 * is non-zero and plate is populated when command is one of the
 * HIK_COMM_*PLATE* codes above and the SDK payload could be decoded;
 * otherwise raw/raw_len give the undecoded payload for the command (a
 * generic escape hatch - see HCNetSDK.h for the struct matching a given
 * lCommand). All pointers are only valid for the duration of the call. Call
 * once per process. */
int32_t hik_set_alarm_callback(void);

int32_t hik_alarm_chan_open(int32_t user_id);
int32_t hik_alarm_chan_close(int32_t alarm_handle);

/* ================================= ANPR ================================= */

int32_t hik_manual_snap(int32_t user_id, int32_t channel, hik_plate_event *out,
                         uint8_t *scene_buf, uint32_t scene_buf_cap, uint32_t *scene_len,
                         uint8_t *plate_buf, uint32_t plate_buf_cap, uint32_t *plate_len);

/* ============================= generic config ============================= */

/* NET_DVR_STDXMLConfig - generic ISAPI-style passthrough. url is an ISAPI-style
 * path such as "PUT /ISAPI/Traffic/channels/1/vehiclePositionControl?format=json".
 * in_buf/in_len is the request body (XML or JSON), out_buf/out_buf_cap is
 * caller-owned and receives the response body, *out_len receives the number of
 * bytes written. */
int32_t hik_stdxml_config(int32_t user_id, const char *url,
                           const uint8_t *in_buf, uint32_t in_len,
                           uint8_t *out_buf, uint32_t out_buf_cap, uint32_t *out_len);

/* NET_DVR_GetDVRConfig / NET_DVR_SetDVRConfig - legacy binary command+struct
 * passthrough for any command not covered by a dedicated wrapper above. The
 * caller is responsible for knowing the exact struct layout for dwCommand
 * (see HCNetSDK.h) and building/parsing buf accordingly. */
int32_t hik_get_dvr_config(int32_t user_id, uint32_t command, int32_t channel,
                            uint8_t *out_buf, uint32_t out_buf_cap, uint32_t *out_len);
int32_t hik_set_dvr_config(int32_t user_id, uint32_t command, int32_t channel,
                            const uint8_t *in_buf, uint32_t in_len);

#ifdef __cplusplus
}
#endif

#endif /* HIKGO_SHIM_H */
