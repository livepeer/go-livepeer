// Minimal CRC32 implementation
function crc32(buf) {
  const table = crc32.table || (crc32.table = (function() {
    const t = new Uint32Array(256);
    for (let n = 0; n < 256; n++) {
      let c = n;
      for (let k = 0; k < 8; k++) {
        c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1);
      }
      t[n] = c;
    }
    return t;
  })());
  let crc = 0xFFFFFFFF;
  for (const b of buf) crc = (crc >>> 8) ^ table[(crc ^ b) & 0xFF];
  return (crc ^ 0xFFFFFFFF) >>> 0;
}

class TSMuxer {
  constructor() {
    this.packetSize = 188;
    this.patPid = 0x0000;
    this.pmtPid = 0x0100;
    this.videoPid = 0x0101;
    this.audioPid = 0x0102;
    this.streamIdVideo = 0xE0;
    this.streamIdAudio = 0xC0;
    this.cc = new Map(); // pid -> next value 0..15
    this.onData = (packet) => { /* override to handle output */ };
    this.initTables();
  }

  initTables() {
    this.patPackets = this.createPAT();
    this.pmtPackets = this.createPMT();
    // emit PAT and PMT at start
    this.patPackets.forEach(pkt => this.onData(pkt));
    this.pmtPackets.forEach(pkt => this.onData(pkt));
  }

  createPacket(payload, pid, payloadUnitStart = false, addPointer = false) {
    const pkt = new Uint8Array(this.packetSize).fill(0xFF);
    pkt[0] = 0x47;
    pkt[1] = (payloadUnitStart ? 0x40 : 0x00) | ((pid >> 8) & 0x1F);
    pkt[2] = pid & 0xFF;


    /* ---------- continuity counter ---------- */
    const cc = this.cc.get(pid) ?? 0;
    let afc = 1;                                 // ‘01’ = payload only
    let off = 4;                                 // cursor after TS header
    this.cc.set(pid, (cc + 1) & 0x0F);

    /* ---------- optional pointer field (PAT/PMT only) ---------- */
    if (payloadUnitStart && addPointer) {
      pkt[off++] = 0x00;
    }

    /* ---------- decide if we need stuffing ---------- */
    const bytesLeft = 188 - off;
    if (payload.length < bytesLeft) {
      afc = 3;                                 // ‘11’ = adaptation + payload
      const stuffing = bytesLeft - payload.length - 2; // 2 = len + flags
      pkt[off++] = stuffing + 1;               // adaptation_field_length
      pkt[off++] = 0x00;                       // flags byte (all zero)
      if (stuffing > 0) pkt.fill(0xFF, off, off + stuffing);
      off += stuffing;                         // now ‘off’ points to payload
    }

    pkt[3] = (afc << 4) | (cc & 0x0F);           // finish byte 3
    pkt.set(payload.subarray(0, 188 - off), off);
    return pkt;
  }

  createPAT() {
    const section = [];
    section.push(0x00);                    // table_id
    section.push(0xB0, 0x0D);              // section_syntax, length=13
    section.push(0x00, 0x01);              // transport_stream_id = 1
    section.push(0xC1);                    // version=0, current_next
    section.push(0x00, 0x00);              // section_number, last_section
    section.push(0x00, 0x01);              // program_number = 1
    section.push(0xE0 | (this.pmtPid >> 8), this.pmtPid & 0xFF);
    const crc = crc32(Uint8Array.from(section));
    section.push((crc >> 24)&0xFF,(crc >> 16)&0xFF,(crc >> 8)&0xFF,crc & 0xFF);
    const payload = Uint8Array.from(section);
    return [ this.createPacket(payload, this.patPid, true, true) ];
  }

  createPMT() {
    let section = [];
    section.push(0x02);                    // table_id = PMT
    section.push(0xB0, 0x00);              // placeholder for section_length
    section.push(0x00, 0x01);              // program_number = 1
    section.push(0xC1);                    // version=0, current_next
    section.push(0x00, 0x00);              // section_number, last_section
    section.push(0xE0 | (this.videoPid >> 8), this.videoPid & 0xFF); // PCR PID
    section.push(0xF0, 0x00);              // program_info_length = 0
    // Video stream descriptor
    section.push(0x1B);                     // stream_type = H.264 video
    section.push(0xE0 | (this.videoPid >> 8), this.videoPid & 0xFF);
    section.push(0xF0, 0x00);
    // Audio stream descriptor
    section.push(0x0F);                     // stream_type = AAC
    section.push(0xE0 | (this.audioPid >> 8), this.audioPid & 0xFF);
    section.push(0xF0, 0x00);
    // compute and patch section_length
    const len = section.length + 4 - 3;
    section[1] = 0xB0 | ((len >> 8) & 0x0F);
    section[2] = len & 0xFF;
    const payload = Uint8Array.from(section);
    const crc = crc32(payload);
    const full = new Uint8Array(payload.length + 4);
    full.set(payload, 0);
    full.set([(crc>>24)&0xFF,(crc>>16)&0xFF,(crc>>8)&0xFF,crc&0xFF], payload.length);
    return [ this.createPacket(full, this.pmtPid, true, true) ];
  }

  muxPacket(isVideo, pts, dts, data) {
    const pid = isVideo ? this.videoPid : this.audioPid;
    const streamId = isVideo ? this.streamIdVideo : this.streamIdAudio;
    // Build PES header
    const header = [];
    header.push(0x00, 0x00, 0x01, streamId);
    let pesLen = data.length + 8;
    if (isVideo || pesLen > 0xFFFF) {
      pesLen = 0; // until next start code
    }
    header.push((pesLen>>8)&0xFF, pesLen&0xFF);

    // flags: PTS only, header length=5
    //    NB: with bframes, flags=0xC0 and header-length=10
    header.push(0x80, 0x80, 0x05);

    // write 33-bit PTS
    const ptsVal = pts;
    header.push((0x02<<4)|(((ptsVal>>30)&0x07)<<1)|1);
    header.push((ptsVal>>22)&0xFF);
    header.push((((ptsVal>>15)&0x7F)<<1)|1);
    header.push((ptsVal>>7)&0xFF);
    header.push(((ptsVal&0x7F)<<1)|1);
    const pes = new Uint8Array(header.length + data.length);
    pes.set(header, 0);
    pes.set(data, header.length);
    // split into TS packets
    let offset = 0;
    let first = true;
    while (offset < pes.length) {
      const sliceLen = Math.min(pes.length - offset, this.packetSize - 4);
      const slice = pes.subarray(offset, offset + sliceLen);
      const pkt = this.createPacket(slice, pid, first, false);
      this.onData(pkt);
      first = false;
      offset += sliceLen;
    }
  }
}

export default TSMuxer;
