#include "types.hpp"
#include <cstring>
#include <algorithm>
#include <chrono>
#include <random>
#include <fstream>
#include <iostream>

// CRC32 table for fast calculation
static const uint32_t crc32_table[256] = {
    0x00000000, 0x77073096, 0xEE0E612C, 0x990951BA, 0x076DC419, 0x706AF48F, 0xE963A535, 0x9E6495A3,
    0x0EDB8832, 0x79DCB8A4, 0xE0D5E91E, 0x97D2D988, 0x09B64C2B, 0x7EB17CBD, 0xE7B82D07, 0x90BF1D91,
    0x1DB71064, 0x6AB020F2, 0xF3B97148, 0x84BE41DE, 0x1ADAD47D, 0x6DDDE4EB, 0xF4D4B551, 0x83D385C7,
    0x136C9856, 0x646BA8C0, 0xFD62F97A, 0x8A65C9EC, 0x14015C4F, 0x63066CD9, 0xFA0F3D63, 0x8D080DF5,
    0x3B6E20C8, 0x4C69105E, 0xD56041E4, 0xA2677172, 0x3C03E4D1, 0x4B04D447, 0xD20D85FD, 0xA50AB56B,
    0x35B5A8FA, 0x42B2986C, 0xDBBBC9D6, 0xACBCF940, 0x32D86CE3, 0x45DF5C75, 0xDCD60DCF, 0xABD13D59,
    0x26D930AC, 0x51DE003A, 0xC8D75180, 0xBFD06116, 0x21B4F4B5, 0x56B3C423, 0xCFBA9599, 0xB8BDA50F,
    0x2802B89E, 0x5F058808, 0xC60CD9B2, 0xB10BE924, 0x2F6F7C87, 0x58684C11, 0xC1611DAB, 0xB6662D3D,
    0x76DC4190, 0x01DB7106, 0x98D220BC, 0xEFD5102A, 0x71B18589, 0x06B6B51F, 0x9FBFE4A5, 0xE8B8D433,
    0x7807C9A2, 0x0F00F934, 0x9609A88E, 0xE10E9818, 0x7F6A0DBB, 0x086D3D2D, 0x91646C97, 0xE6635C01,
    0x6B6B51F4, 0x1C6C6162, 0x856530D8, 0xF262004E, 0x6C0695ED, 0x1B01A57B, 0x8208F4C1, 0xF50FC457,
    0x65B0D9C6, 0x12B7E950, 0x8BBEB8EA, 0xFCB9887C, 0x62DD1DDF, 0x15DA2D49, 0x8CD37CF3, 0xFBD44C65,
    0x4DB26158, 0x3AB551CE, 0xA3BC0074, 0xD4BB30E2, 0x4ADFA541, 0x3DD895D7, 0xA4D1C46D, 0xD3D6F4FB,
    0x4369E96A, 0x346ED9FC, 0xAD678846, 0xDA60B8D0, 0x44042D73, 0x33031DE5, 0xAA0A4C5F, 0xDD0D7CC9,
    0x5005713C, 0x270241AA, 0xBE0B1010, 0xC90C2086, 0x5768B525, 0x206F85B3, 0xB966D409, 0xCE61E49F,
    0x5EDEF90E, 0x29D9C998, 0xB0D09822, 0xC7D7A8B4, 0x59B33D17, 0x2EB40D81, 0xB7BD5C3B, 0xC0BA6CAD,
    0xEDB88320, 0x9ABFB3B6, 0x03B6E20C, 0x74B1D29A, 0xEAD54739, 0x9DD277AF, 0x04DB2615, 0x73DC1683,
    0xE3630B12, 0x94643B84, 0x0D6D6A3E, 0x7A6A5AA8, 0xE40ECF0B, 0x9309FF9D, 0x0A00AE27, 0x7D079EB1,
    0xF00F9344, 0x8708A3D2, 0x1E01F268, 0x6906C2FE, 0xF762575D, 0x806567CB, 0x196C3671, 0x6E6B06E7,
    0xFED41B76, 0x89D32BE0, 0x10DA7A5A, 0x67DD4ACC, 0xF9B9DF6F, 0x8EBEEFF9, 0x17B7BE43, 0x60B08ED5,
    0xD6D6A3E8, 0xA1D1937E, 0x38D8C2C4, 0x4FDFF252, 0xD1BB67F1, 0xA6BC5767, 0x3FB506DD, 0x48B2364B,
    0xD80D2BDA, 0xAF0A1B4C, 0x36034AF6, 0x41047A60, 0xDF60EFC3, 0xA867DF55, 0x316E8EEF, 0x4669BE79,
    0xCB61B38C, 0xBC66831A, 0x256FD2A0, 0x5268E236, 0xCC0C7795, 0xBB0B4703, 0x220216B9, 0x5505262F,
    0xC5BA3BBE, 0xB2BD0B28, 0x2BB45A92, 0x5CB36A04, 0xC2D7FFA7, 0xB5D0CF31, 0x2CD99E8B, 0x5BDEAE1D,
    0x9B64C2B0, 0xEC63F226, 0x756AA39C, 0x026D930A, 0x9C0906A9, 0xEB0E363F, 0x72076785, 0x05005713,
    0x95BF4A82, 0xE2B87A14, 0x7BB12BAE, 0x0CB61B38, 0x92D28E9B, 0xE5D5BE0D, 0x7CDCEFB7, 0x0BDBDF21,
    0x86D3D2D4, 0xF1D4E242, 0x68DDB3F8, 0x1FDA836E, 0x81BE16CD, 0xF6B9265B, 0x6FB077E1, 0x18B74777,
    0x88085AE6, 0xFF0F6A70, 0x66063BCA, 0x11010B5C, 0x8F659EFF, 0xF862AE69, 0x616BFFD3, 0x166CCF45,
    0xA00AE278, 0xD70DD2EE, 0x4E048354, 0x3903B3C2, 0xA7672661, 0xD06016F7, 0x4969474D, 0x3E6E77DB,
    0xAED16A4A, 0xD9D65ADC, 0x40DF0B66, 0x37D83BF0, 0xA9BCAE53, 0xDEBB9EC5, 0x47B2CF7F, 0x30B5FFE9,
    0xBDBDF21C, 0xCABAC28A, 0x53B39330, 0x24B4A3A6, 0xBAD03605, 0xCDD70693, 0x54DE5729, 0x23D967BF,
    0xB3667A2E, 0xC4614AB8, 0x5D681B02, 0x2A6F2B94, 0xB40BBE37, 0xC30C8EA1, 0x5A05DF1B, 0x2D02EF8D
};

namespace hts {

uint32_t calculate_crc32(const std::vector<uint8_t>& data) {
    uint32_t crc = 0xFFFFFFFF;
    for (uint8_t byte : data) {
        crc = crc32_table[(crc ^ byte) & 0xFF] ^ (crc >> 8);
    }
    return crc ^ 0xFFFFFFFF;
}

std::vector<uint8_t> serialize_packet(const Packet& packet) {
    std::vector<uint8_t> data;
    data.reserve(PACKET_HEADER_SIZE + packet.payload.size());
    
    // Serialize header (big-endian)
    auto add_uint32 = [&data](uint32_t value) {
        data.push_back((value >> 24) & 0xFF);
        data.push_back((value >> 16) & 0xFF);
        data.push_back((value >> 8) & 0xFF);
        data.push_back(value & 0xFF);
    };
    
    auto add_uint64 = [&data](uint64_t value) {
        data.push_back((value >> 56) & 0xFF);
        data.push_back((value >> 48) & 0xFF);
        data.push_back((value >> 40) & 0xFF);
        data.push_back((value >> 32) & 0xFF);
        data.push_back((value >> 24) & 0xFF);
        data.push_back((value >> 16) & 0xFF);
        data.push_back((value >> 8) & 0xFF);
        data.push_back(value & 0xFF);
    };
    
    auto add_uint16 = [&data](uint16_t value) {
        data.push_back((value >> 8) & 0xFF);
        data.push_back(value & 0xFF);
    };
    
    add_uint32(packet.header.magic);
    add_uint64(packet.header.sequence_num);
    add_uint32(packet.header.length);
    add_uint32(packet.header.checksum);
    add_uint64(packet.header.timestamp);
    add_uint16(packet.header.flags);
    add_uint16(packet.header.reserved);
    
    // Add payload
    data.insert(data.end(), packet.payload.begin(), packet.payload.end());
    
    return data;
}

Packet deserialize_packet(const std::vector<uint8_t>& data) {
    Packet packet;
    
    if (data.size() < PACKET_HEADER_SIZE) {
        throw std::runtime_error("Invalid packet size");
    }
    
    // Deserialize header (big-endian)
    auto read_uint32 = [&data](size_t offset) -> uint32_t {
        return (static_cast<uint32_t>(data[offset]) << 24) |
               (static_cast<uint32_t>(data[offset + 1]) << 16) |
               (static_cast<uint32_t>(data[offset + 2]) << 8) |
               static_cast<uint32_t>(data[offset + 3]);
    };
    
    auto read_uint64 = [&data](size_t offset) -> uint64_t {
        return (static_cast<uint64_t>(data[offset]) << 56) |
               (static_cast<uint64_t>(data[offset + 1]) << 48) |
               (static_cast<uint64_t>(data[offset + 2]) << 40) |
               (static_cast<uint64_t>(data[offset + 3]) << 32) |
               (static_cast<uint64_t>(data[offset + 4]) << 24) |
               (static_cast<uint64_t>(data[offset + 5]) << 16) |
               (static_cast<uint64_t>(data[offset + 6]) << 8) |
               static_cast<uint64_t>(data[offset + 7]);
    };
    
    auto read_uint16 = [&data](size_t offset) -> uint16_t {
        return (static_cast<uint16_t>(data[offset]) << 8) |
               static_cast<uint16_t>(data[offset + 1]);
    };
    
    packet.header.magic = read_uint32(0);
    packet.header.sequence_num = read_uint64(4);
    packet.header.length = read_uint32(12);
    packet.header.checksum = read_uint32(16);
    packet.header.timestamp = read_uint64(20);
    packet.header.flags = read_uint16(28);
    packet.header.reserved = read_uint16(30);
    
    // Extract payload
    if (data.size() >= PACKET_HEADER_SIZE + packet.header.length) {
        packet.payload.assign(data.begin() + PACKET_HEADER_SIZE, 
                             data.begin() + PACKET_HEADER_SIZE + packet.header.length);
    }
    
    return packet;
}

bool validate_packet(const Packet& packet) {
    // Check magic number
    if (packet.header.magic != DEFAULT_MAGIC) {
        return false;
    }
    
    // Check packet size
    if (packet.header.length > MAX_PACKET_SIZE) {
        return false;
    }
    
    // Check payload length matches header
    if (packet.payload.size() != packet.header.length) {
        return false;
    }
    
    // Verify checksum
    uint32_t calculated_checksum = calculate_crc32(packet.payload);
    if (calculated_checksum != packet.header.checksum) {
        return false;
    }
    
    return true;
}

uint64_t get_current_timestamp() {
    auto now = std::chrono::high_resolution_clock::now();
    auto duration = now.time_since_epoch();
    return std::chrono::duration_cast<std::chrono::nanoseconds>(duration).count();
}

double calculate_throughput(uint64_t bytes, std::chrono::milliseconds duration) {
    if (duration.count() == 0) {
        return 0.0;
    }
    return static_cast<double>(bytes) / (duration.count() / 1000.0);
}

double convert_to_gbps(double bytes_per_second) {
    return bytes_per_second * 8.0 / (1024.0 * 1024.0 * 1024.0);
}

double convert_from_gbps(double gbps) {
    return gbps * 1024.0 * 1024.0 * 1024.0 / 8.0;
}

Packet create_packet(uint64_t sequence_num, const std::vector<uint8_t>& payload, uint16_t flags) {
    Packet packet;
    packet.header.magic = DEFAULT_MAGIC;
    packet.header.sequence_num = sequence_num;
    packet.header.length = static_cast<uint32_t>(payload.size());
    packet.header.checksum = calculate_crc32(payload);
    packet.header.timestamp = get_current_timestamp();
    packet.header.flags = flags;
    packet.header.reserved = 0;
    packet.payload = payload;
    return packet;
}

std::vector<uint8_t> generate_random_data(size_t size) {
    std::vector<uint8_t> data(size);
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<> dis(0, 255);
    
    for (size_t i = 0; i < size; ++i) {
        data[i] = static_cast<uint8_t>(dis(gen));
    }
    
    return data;
}

bool file_exists(const std::string& filename) {
    std::ifstream file(filename);
    return file.good();
}

uint64_t get_file_size(const std::string& filename) {
    std::ifstream file(filename, std::ios::binary | std::ios::ate);
    if (!file.is_open()) {
        return 0;
    }
    return static_cast<uint64_t>(file.tellg());
}

std::string get_error_string(ErrorCode code) {
    switch (code) {
        case ErrorCode::SUCCESS: return "Success";
        case ErrorCode::INVALID_PACKET_SIZE: return "Invalid packet size";
        case ErrorCode::INVALID_MAGIC: return "Invalid magic number";
        case ErrorCode::PACKET_TOO_LARGE: return "Packet too large";
        case ErrorCode::CHECKSUM_MISMATCH: return "Checksum mismatch";
        case ErrorCode::TIMEOUT: return "Operation timeout";
        case ErrorCode::CONNECTION_CLOSED: return "Connection closed";
        case ErrorCode::INVALID_SEQUENCE: return "Invalid sequence number";
        case ErrorCode::BUFFER_FULL: return "Buffer full";
        case ErrorCode::FEC_DECODE_FAILED: return "FEC decode failed";
        case ErrorCode::FILE_NOT_FOUND: return "File not found";
        case ErrorCode::INVALID_FILE_FORMAT: return "Invalid file format";
        case ErrorCode::STREAM_CLOSED: return "Stream closed";
        case ErrorCode::LINK_INTERRUPTED: return "Link interrupted";
        case ErrorCode::LINK_TIMEOUT: return "Link timeout";
        case ErrorCode::RESYNC_FAILED: return "Resync failed";
        case ErrorCode::INVALID_UDP_PAYLOAD: return "Invalid UDP payload";
        case ErrorCode::INVALID_DATA_SINK: return "Invalid data sink";
        case ErrorCode::FILE_ROTATION_FAILED: return "File rotation failed";
        case ErrorCode::DIRECTORY_NOT_FOUND: return "Directory not found";
        default: return "Unknown error";
    }
}

} // namespace hts