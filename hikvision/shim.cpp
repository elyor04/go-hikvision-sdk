// shim.cpp - the only translation unit in this package that includes the real
// HCNetSDK.h. See shim.h for why. Compiled as C++ (cgo routes .cpp files to
// CXX automatically); everything it exports is plain-C (see shim.h).
#include <cstring>
#include <cstdlib>

#include "HCNetSDK.h"
#include "shim.h"

namespace {

void copy_time_in(const hik_time &in, NET_DVR_TIME &out) {
    std::memset(&out, 0, sizeof(out));
    out.dwYear = in.year;
    out.dwMonth = in.month;
    out.dwDay = in.day;
    out.dwHour = in.hour;
    out.dwMinute = in.minute;
    out.dwSecond = in.second;
}

void copy_time_search_cond(const hik_time &in, NET_DVR_TIME_SEARCH_COND &out) {
    std::memset(&out, 0, sizeof(out));
    out.wYear = in.year;
    out.byMonth = in.month;
    out.byDay = in.day;
    out.byHour = in.hour;
    out.byMinute = in.minute;
    out.bySecond = in.second;
}

void copy_time_search_out(const NET_DVR_TIME_SEARCH &in, hik_time &out) {
    out.year = in.wYear;
    out.month = in.byMonth;
    out.day = in.byDay;
    out.hour = in.byHour;
    out.minute = in.byMinute;
    out.second = in.bySecond;
    out.tz_offset_hour = in.cTimeDifferenceH;
    out.tz_offset_min = in.cTimeDifferenceM;
    out.tz_valid = in.byLocalOrUTC;
}

void fill_alarmer(const NET_DVR_ALARMER *in, hik_alarmer &out) {
    std::memset(&out, 0, sizeof(out));
    if (!in) {
        return;
    }
    out.user_id_valid = in->byUserIDValid;
    out.user_id = in->lUserID;
    out.device_ip_valid = in->byDeviceIPValid;
    std::memcpy(out.device_ip, in->sDeviceIP, sizeof(out.device_ip) < sizeof(in->sDeviceIP) ? sizeof(out.device_ip) : sizeof(in->sDeviceIP));
    out.serial_valid = in->bySerialValid;
    std::memcpy(out.serial_number, in->sSerialNumber, sizeof(out.serial_number) < sizeof(in->sSerialNumber) ? sizeof(out.serial_number) : sizeof(in->sSerialNumber));
}

void fill_plate_from_its(const NET_ITS_PLATE_RESULT *its, hik_plate_event &out) {
    std::memset(&out, 0, sizeof(out));
    std::memcpy(out.license, its->struPlateInfo.sLicense,
                sizeof(out.license) - 1 < sizeof(its->struPlateInfo.sLicense) ? sizeof(out.license) - 1 : sizeof(its->struPlateInfo.sLicense));
    out.confidence = its->struPlateInfo.byEntireBelieve;
    out.plate_color = its->struPlateInfo.byColor;
    out.vehicle_color = its->struVehicleInfo.byColor;
    // its->byVehicleType (not struVehicleInfo.byVehicleType) is the field
    // HCNetSDK.h documents as "refer to VTR_RESULT" - struVehicleInfo's is an
    // undocumented, different/coarser scale (confirmed on real hardware: a
    // car reported byVehicleType=3/VTR_RESULT_CAR at the top level but
    // struVehicleInfo.byVehicleType=1/VTR_RESULT_BUS).
    out.vehicle_type = its->byVehicleType;
    out.speed_kmh = its->struVehicleInfo.wSpeed;
    out.direction = its->byDir;
    out.country = its->struPlateInfo.byCountry;
    out.lane = its->byDriveChan;
    if (its->struPlateInfo.pXmlBuf && its->struPlateInfo.dwXmlLen) {
        out.raw_xml = reinterpret_cast<const uint8_t *>(its->struPlateInfo.pXmlBuf);
        out.raw_xml_len = its->struPlateInfo.dwXmlLen;
    }
    copy_time_search_out(NET_DVR_TIME_SEARCH{
                              its->struSnapFirstPicTime.wYear, its->struSnapFirstPicTime.byMonth,
                              its->struSnapFirstPicTime.byDay, its->struSnapFirstPicTime.byHour,
                              its->struSnapFirstPicTime.byMinute, its->struSnapFirstPicTime.bySecond,
                              its->struSnapFirstPicTime.cTimeDifferenceH, its->struSnapFirstPicTime.cTimeDifferenceM,
                              its->struSnapFirstPicTime.byISO8601, its->struSnapFirstPicTime.wMilliSec},
                          out.capture_time);

    DWORD n = its->dwPicNum;
    if (n > 6) n = 6;
    for (DWORD i = 0; i < n; i++) {
        const NET_ITS_PICTURE_INFO &pic = its->struPicInfo[i];
        if (pic.byType == 0 && out.plate_image == NULL) {
            out.plate_image = pic.pBuffer;
            out.plate_image_len = pic.dwDataLen;
        } else if (pic.byType == 1 && out.scene_image == NULL) {
            out.scene_image = pic.pBuffer;
            out.scene_image_len = pic.dwDataLen;
        }
    }
}

void fill_plate_from_result(const NET_DVR_PLATE_RESULT *r, hik_plate_event &out) {
    std::memset(&out, 0, sizeof(out));
    std::memcpy(out.license, r->struPlateInfo.sLicense,
                sizeof(out.license) - 1 < sizeof(r->struPlateInfo.sLicense) ? sizeof(out.license) - 1 : sizeof(r->struPlateInfo.sLicense));
    out.confidence = r->struPlateInfo.byEntireBelieve;
    out.plate_color = r->struPlateInfo.byColor;
    out.vehicle_color = r->struVehicleInfo.byColor;
    // r->byVehicleType (not struVehicleInfo.byVehicleType) is the field
    // HCNetSDK.h documents as "refer to VTR_RESULT" - see the identical
    // comment in fill_plate_from_its, confirmed on real hardware.
    out.vehicle_type = r->byVehicleType;
    out.speed_kmh = r->struVehicleInfo.wSpeed;
    out.country = r->struPlateInfo.byCountry;
    out.lane = r->byDriveChan;
    // direction is left 0 - NET_DVR_PLATE_RESULT has no byDir field (that's
    // only on the newer NET_ITS_PLATE_RESULT, see fill_plate_from_its).
    if (r->struPlateInfo.pXmlBuf && r->struPlateInfo.dwXmlLen) {
        out.raw_xml = reinterpret_cast<const uint8_t *>(r->struPlateInfo.pXmlBuf);
        out.raw_xml_len = r->struPlateInfo.dwXmlLen;
    }
    if (r->pBuffer2 && r->dwPicLen) {
        out.scene_image = r->pBuffer2;
        out.scene_image_len = r->dwPicLen;
    }
    // pBuffer2 holds the scene snapshot followed immediately by the license
    // plate snapshot (see the comment after NET_DVR_PLATE_RESULT's definition
    // in HCNetSDK.h) - the plate crop is the dwPicPlateLen bytes right after
    // the scene image's dwPicLen bytes, not a separate buffer.
    if (r->pBuffer2 && r->dwPicPlateLen) {
        out.plate_image = r->pBuffer2 + r->dwPicLen;
        out.plate_image_len = r->dwPicPlateLen;
    }
}

} // namespace

// Forward declarations of the cgo-generated C symbols for this package's
// //export'd Go functions (callbacks.go). cgo cannot express "take the
// address of my own //export function" from Go code (there is no way to
// spell `C.goRealDataTrampoline` as a function-pointer value on the Go
// side), so instead these are declared here with C linkage and called
// directly - the real definitions are emitted by cgo into the final linked
// binary. Signatures must match callbacks.go's //export functions exactly.
extern "C" {
/* user is a runtime/cgo.Handle value round-tripped as a plain integer
 * (uintptr_t), not a real pointer - see callbacks.go/preview.go/playback.go.
 * HCNetSDK's own callbacks still take a real void* pUser, so the trampolines
 * below cast between the two right at the SDK boundary. */
void goRealDataTrampoline(int32_t play_handle, uint32_t data_type, const uint8_t *buffer, uint32_t buf_size, uintptr_t user);
void goPlaybackDataTrampoline(int32_t play_handle, uint32_t data_type, const uint8_t *buffer, uint32_t buf_size, uintptr_t user);
void goExceptionTrampoline(uint32_t exception_type, int32_t user_id, int32_t handle, void *user);
void goAlarmTrampoline(int32_t command, const hik_alarmer *alarmer, const uint8_t *raw, uint32_t raw_len,
                        int32_t is_plate, const hik_plate_event *plate, void *user);
}

namespace {

extern "C" void CALLBACK realdata_trampoline(LONG lRealHandle, DWORD dwDataType, BYTE *pBuffer, DWORD dwBufSize, void *pUser) {
    goRealDataTrampoline(lRealHandle, dwDataType, reinterpret_cast<const uint8_t *>(pBuffer), dwBufSize, reinterpret_cast<uintptr_t>(pUser));
}

extern "C" void CALLBACK playback_trampoline(LONG lPlayHandle, DWORD dwDataType, BYTE *pBuffer, DWORD dwBufSize, void *pUser) {
    goPlaybackDataTrampoline(lPlayHandle, dwDataType, reinterpret_cast<const uint8_t *>(pBuffer), dwBufSize, reinterpret_cast<uintptr_t>(pUser));
}

extern "C" void CALLBACK exception_trampoline(DWORD dwType, LONG lUserID, LONG lHandle, void *pUser) {
    goExceptionTrampoline(dwType, lUserID, lHandle, pUser);
}

extern "C" void CALLBACK alarm_trampoline(LONG lCommand, NET_DVR_ALARMER *pAlarmer, char *pAlarmInfo, DWORD dwBufLen, void *pUser) {
    hik_alarmer alarmer;
    fill_alarmer(pAlarmer, alarmer);

    int32_t is_plate = 0;
    hik_plate_event plate;
    std::memset(&plate, 0, sizeof(plate));

    if (pAlarmInfo != NULL) {
        if (lCommand == HIK_COMM_ITS_PLATE_RESULT && dwBufLen >= sizeof(NET_ITS_PLATE_RESULT)) {
            fill_plate_from_its(reinterpret_cast<const NET_ITS_PLATE_RESULT *>(pAlarmInfo), plate);
            is_plate = 1;
        } else if ((lCommand == HIK_COMM_UPLOAD_PLATE_RESULT || lCommand == HIK_COMM_PLATE_RESULT_V50) &&
                   dwBufLen >= sizeof(NET_DVR_PLATE_RESULT)) {
            fill_plate_from_result(reinterpret_cast<const NET_DVR_PLATE_RESULT *>(pAlarmInfo), plate);
            is_plate = 1;
        }
    }

    goAlarmTrampoline(lCommand, &alarmer, reinterpret_cast<const uint8_t *>(pAlarmInfo), dwBufLen,
                       is_plate, is_plate ? &plate : NULL, pUser);
}

} // namespace

extern "C" {

/* ============================== lifecycle ============================== */

int32_t hik_init(void) { return NET_DVR_Init() ? 0 : -1; }
int32_t hik_cleanup(void) { return NET_DVR_Cleanup() ? 0 : -1; }
uint32_t hik_get_sdk_version(void) { return NET_DVR_GetSDKVersion(); }
uint32_t hik_get_sdk_build_version(void) { return NET_DVR_GetSDKBuildVersion(); }

int32_t hik_set_sdk_component_path(const char *path) {
    NET_DVR_LOCAL_SDK_PATH cfg;
    std::memset(&cfg, 0, sizeof(cfg));
    std::strncpy(cfg.sPath, path, sizeof(cfg.sPath) - 1);
    return NET_DVR_SetSDKInitCfg(NET_SDK_INIT_CFG_SDK_PATH, &cfg) ? 0 : -1;
}

int32_t hik_set_connect_time(uint32_t wait_time_ms, uint32_t try_times) {
    return NET_DVR_SetConnectTime(wait_time_ms, try_times) ? 0 : -1;
}

int32_t hik_set_reconnect(uint32_t interval_ms, int32_t enable) {
    return NET_DVR_SetReconnect(interval_ms, enable ? TRUE : FALSE) ? 0 : -1;
}

int32_t hik_set_log_to_file(int32_t log_level, const char *log_dir, int32_t auto_delete) {
    return NET_DVR_SetLogToFile(log_level, const_cast<char *>(log_dir), auto_delete ? TRUE : FALSE) ? 0 : -1;
}

/* ================================ errors ================================ */

uint32_t hik_get_last_error(void) { return NET_DVR_GetLastError(); }
const char *hik_get_error_msg(void) { return NET_DVR_GetErrorMsg(NULL); }

/* ============================ login / device ============================= */

int32_t hik_login(const hik_login_info *in, hik_device_info *out) {
    NET_DVR_USER_LOGIN_INFO login;
    std::memset(&login, 0, sizeof(login));
    std::strncpy(login.sDeviceAddress, in->device_address, sizeof(login.sDeviceAddress) - 1);
    login.wPort = in->port;
    std::strncpy(login.sUserName, in->user_name, sizeof(login.sUserName) - 1);
    std::strncpy(login.sPassword, in->password, sizeof(login.sPassword) - 1);
    login.bUseAsynLogin = FALSE;
    login.byHttps = static_cast<BYTE>(in->use_https);

    NET_DVR_DEVICEINFO_V40 dev;
    std::memset(&dev, 0, sizeof(dev));

    LONG userID = NET_DVR_Login_V40(&login, &dev);
    if (out) {
        std::memset(out, 0, sizeof(*out));
        std::memcpy(out->serial_number, dev.struDeviceV30.sSerialNumber,
                    sizeof(out->serial_number) < sizeof(dev.struDeviceV30.sSerialNumber) ? sizeof(out->serial_number) : sizeof(dev.struDeviceV30.sSerialNumber));
        out->alarm_in_port_num = dev.struDeviceV30.byAlarmInPortNum;
        out->alarm_out_port_num = dev.struDeviceV30.byAlarmOutPortNum;
        out->disk_num = dev.struDeviceV30.byDiskNum;
        out->dvr_type = dev.struDeviceV30.byDVRType;
        out->chan_num = dev.struDeviceV30.byChanNum;
        out->start_chan = dev.struDeviceV30.byStartChan;
        out->ip_chan_num_low = dev.struDeviceV30.byIPChanNum;
        out->start_dchan = dev.struDeviceV30.byStartDChan;
        out->high_dchan_num = dev.struDeviceV30.byHighDChanNum;
        out->ip_chan_num = static_cast<uint32_t>(dev.struDeviceV30.byIPChanNum) +
                            static_cast<uint32_t>(dev.struDeviceV30.byHighDChanNum) * 256u;
        out->dev_type = dev.struDeviceV30.wDevType;
    }
    return userID;
}

int32_t hik_logout(int32_t user_id) { return NET_DVR_Logout_V30(user_id) ? 0 : -1; }

/* ================================ preview ================================ */

int32_t hik_realplay_start(int32_t user_id, int32_t channel, uint32_t stream_type, uintptr_t user) {
    NET_DVR_PREVIEWINFO info;
    std::memset(&info, 0, sizeof(info));
    info.lChannel = channel;
    info.dwStreamType = stream_type;
    info.dwLinkMode = 0; /* TCP */
    info.hPlayWnd = NULL;
    info.bBlocked = 1;
    info.byProtoType = 0;

    return NET_DVR_RealPlay_V40(user_id, &info, realdata_trampoline, reinterpret_cast<void *>(user));
}

int32_t hik_realplay_stop(int32_t real_handle) { return NET_DVR_StopRealPlay(real_handle) ? 0 : -1; }

int32_t hik_set_exception_callback(void) {
    return NET_DVR_SetExceptionCallBack_V30(0, NULL, exception_trampoline, NULL) ? 0 : -1;
}

/* ================================== PTZ ================================== */

int32_t hik_ptz_control(int32_t user_id, int32_t channel, uint32_t command, uint32_t stop, uint32_t speed) {
    return NET_DVR_PTZControlWithSpeed_Other(user_id, channel, command, stop, speed) ? 0 : -1;
}

int32_t hik_ptz_preset(int32_t user_id, int32_t channel, uint32_t preset_cmd, uint32_t preset_index) {
    return NET_DVR_PTZPreset_Other(user_id, channel, preset_cmd, preset_index) ? 0 : -1;
}

/* ================================ capture ================================ */

int32_t hik_capture_jpeg(int32_t user_id, int32_t channel, uint16_t pic_size, uint16_t jpeg_quality,
                          uint8_t *buf, uint32_t buf_cap, uint32_t *out_len) {
    NET_DVR_JPEGPARA para;
    std::memset(&para, 0, sizeof(para));
    para.wPicSize = pic_size;
    para.wPicQuality = jpeg_quality;

    DWORD written = 0;
    BOOL ok = NET_DVR_CaptureJPEGPicture_NEW(user_id, channel, &para, reinterpret_cast<char *>(buf), buf_cap, &written);
    if (out_len) *out_len = written;
    return ok ? 0 : -1;
}

/* ================================ playback ================================ */

int32_t hik_find_file_open(int32_t user_id, int32_t channel, hik_time start, hik_time stop) {
    NET_DVR_FILECOND_V50 cond;
    std::memset(&cond, 0, sizeof(cond));
    cond.struStreamID.dwChannel = channel;
    cond.byStreamType = 0xff; /* all */
    copy_time_search_cond(start, cond.struStartTime);
    copy_time_search_cond(stop, cond.struStopTime);
    return NET_DVR_FindFile_V50(user_id, &cond);
}

int32_t hik_find_file_next(int32_t find_handle, hik_find_data *out) {
    NET_DVR_FINDDATA_V50 data;
    std::memset(&data, 0, sizeof(data));
    LONG rc = NET_DVR_FindNextFile_V50(find_handle, &data);
    if (rc == NET_DVR_FILE_SUCCESS && out) {
        std::memset(out, 0, sizeof(*out));
        std::strncpy(out->file_name, data.sFileName, sizeof(out->file_name) - 1);
        copy_time_search_out(data.struStartTime, out->start_time);
        copy_time_search_out(data.struStopTime, out->stop_time);
        out->file_size = data.dwFileSize;
        out->locked = data.byLocked;
        out->file_type = data.byFileType;
    }
    return rc;
}

int32_t hik_find_file_close(int32_t find_handle) { return NET_DVR_FindClose_V30(find_handle) ? 0 : -1; }

int32_t hik_playback_start_by_time(int32_t user_id, int32_t channel, hik_time start, hik_time stop, uintptr_t user) {
    NET_DVR_VOD_PARA vod;
    std::memset(&vod, 0, sizeof(vod));
    vod.dwSize = sizeof(vod);
    vod.struIDInfo.dwChannel = channel;
    copy_time_in(start, vod.struBeginTime);
    copy_time_in(stop, vod.struEndTime);
    vod.hWnd = NULL;

    LONG handle = NET_DVR_PlayBackByTime_V40(user_id, &vod);
    if (handle >= 0) {
        NET_DVR_SetPlayDataCallBack_V40(handle, playback_trampoline, reinterpret_cast<void *>(user));
    }
    return handle;
}

int32_t hik_playback_control(int32_t play_handle, uint32_t control_code) {
    return NET_DVR_PlayBackControl_V40(play_handle, control_code, NULL, 0, NULL, NULL) ? 0 : -1;
}

int32_t hik_playback_stop(int32_t play_handle) { return NET_DVR_StopPlayBack(play_handle) ? 0 : -1; }

int32_t hik_download_start_by_time(int32_t user_id, int32_t channel, hik_time start, hik_time stop,
                                    const char *saved_file_name) {
    NET_DVR_PLAYCOND cond;
    std::memset(&cond, 0, sizeof(cond));
    cond.dwChannel = channel;
    copy_time_in(start, cond.struStartTime);
    copy_time_in(stop, cond.struStopTime);
    return NET_DVR_GetFileByTime_V40(user_id, const_cast<char *>(saved_file_name), &cond);
}

int32_t hik_download_get_progress(int32_t file_handle) { return NET_DVR_GetDownloadPos(file_handle); }
int32_t hik_download_stop(int32_t file_handle) { return NET_DVR_StopGetFile(file_handle) ? 0 : -1; }

/* ================================= alarm ================================= */

int32_t hik_set_alarm_callback(void) {
    return NET_DVR_SetDVRMessageCallBack_V51(0, alarm_trampoline, NULL) ? 0 : -1;
}

int32_t hik_alarm_chan_open(int32_t user_id) {
    NET_DVR_SETUPALARM_PARAM param;
    std::memset(&param, 0, sizeof(param));
    param.dwSize = sizeof(param);
    param.byLevel = 1;
    param.byAlarmInfoType = 1;
    param.byRetAlarmTypeV40 = 1;
    return NET_DVR_SetupAlarmChan_V41(user_id, &param);
}

int32_t hik_alarm_chan_close(int32_t alarm_handle) { return NET_DVR_CloseAlarmChan_V30(alarm_handle) ? 0 : -1; }

/* ================================= ANPR ================================= */

int32_t hik_manual_snap(int32_t user_id, int32_t channel, hik_plate_event *out,
                         uint8_t *scene_buf, uint32_t scene_buf_cap, uint32_t *scene_len,
                         uint8_t *plate_buf, uint32_t plate_buf_cap, uint32_t *plate_len) {
    NET_DVR_MANUALSNAP in;
    std::memset(&in, 0, sizeof(in));
    in.byChannel = static_cast<BYTE>(channel);

    NET_DVR_PLATE_RESULT result;
    std::memset(&result, 0, sizeof(result));
    result.dwSize = sizeof(result);

    BOOL ok = NET_DVR_ManualSnap(user_id, &in, &result);
    if (!ok) {
        return -1;
    }
    if (out) {
        fill_plate_from_result(&result, *out);
    }
    if (scene_buf && result.pBuffer2 && result.dwPicLen) {
        DWORD n = result.dwPicLen < scene_buf_cap ? result.dwPicLen : scene_buf_cap;
        std::memcpy(scene_buf, result.pBuffer2, n);
        if (scene_len) *scene_len = n;
    } else if (scene_len) {
        *scene_len = 0;
    }
    if (plate_buf && result.pBuffer2 && result.dwPicPlateLen) {
        DWORD n = result.dwPicPlateLen < plate_buf_cap ? result.dwPicPlateLen : plate_buf_cap;
        std::memcpy(plate_buf, result.pBuffer2 + result.dwPicLen, n);
        if (plate_len) *plate_len = n;
    } else if (plate_len) {
        *plate_len = 0;
    }
    return 0;
}

/* ============================= generic config ============================= */

int32_t hik_stdxml_config(int32_t user_id, const char *url,
                           const uint8_t *in_buf, uint32_t in_len,
                           uint8_t *out_buf, uint32_t out_buf_cap, uint32_t *out_len) {
    NET_DVR_XML_CONFIG_INPUT in;
    std::memset(&in, 0, sizeof(in));
    in.dwSize = sizeof(in);
    in.lpRequestUrl = const_cast<char *>(url);
    in.dwRequestUrlLen = static_cast<DWORD>(std::strlen(url));
    in.lpInBuffer = const_cast<uint8_t *>(in_buf);
    in.dwInBufferSize = in_len;
    in.byMIMEType = 0;

    NET_DVR_XML_CONFIG_OUTPUT out;
    std::memset(&out, 0, sizeof(out));
    out.dwSize = sizeof(out);
    out.lpOutBuffer = out_buf;
    out.dwOutBufferSize = out_buf_cap;

    BOOL ok = NET_DVR_STDXMLConfig(user_id, &in, &out);
    if (out_len) *out_len = out.dwReturnedXMLSize;
    return ok ? 0 : -1;
}

int32_t hik_get_dvr_config(int32_t user_id, uint32_t command, int32_t channel,
                            uint8_t *out_buf, uint32_t out_buf_cap, uint32_t *out_len) {
    DWORD returned = 0;
    BOOL ok = NET_DVR_GetDVRConfig(user_id, command, channel, out_buf, out_buf_cap, &returned);
    if (out_len) *out_len = returned;
    return ok ? 0 : -1;
}

int32_t hik_set_dvr_config(int32_t user_id, uint32_t command, int32_t channel,
                            const uint8_t *in_buf, uint32_t in_len) {
    return NET_DVR_SetDVRConfig(user_id, command, channel, const_cast<uint8_t *>(in_buf), in_len) ? 0 : -1;
}

} // extern "C"
