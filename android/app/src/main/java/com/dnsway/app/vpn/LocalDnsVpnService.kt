package com.dnsway.app.vpn

import android.net.VpnService
import android.os.ParcelFileDescriptor
import android.util.Log
import com.dnsway.app.engine.Decision
import com.dnsway.app.engine.DnsEngine
import kotlinx.coroutines.*
import org.xbill.DNS.*
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.ConcurrentHashMap

/**
 * Full-tunnel VPN service — routes ALL traffic through the TUN so we can:
 * - Filter DNS (UDP 53) via the local engine
 * - Block DoT (UDP/TCP 853)
 * - Block DoH (TCP 443 → known DoH provider IPs)
 * - Transparently forward everything else
 */
class LocalDnsVpnService : VpnService() {

    private var vpnThread: Thread? = null
    private var vpnInterface: ParcelFileDescriptor? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    private val VPN_IP = "10.1.10.1"
    private val DNS_IP = "10.1.10.2"
    private val UPSTREAM_DNS = "1.1.1.1"

    // TCP connection tracking
    private val tcpSessions = ConcurrentHashMap<Long, TcpSession>()
    private var nextTcpId = 1L

    // UDP sockets for forwarding
    private val udpSockets = hashMapOf<String, DatagramSocket>()

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        Log.i(TAG, "VpnService created")
    }

    private fun createNotificationChannel() {
        if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
            val channel = android.app.NotificationChannel(
                NOTIFICATION_CHANNEL_ID,
                "Dnsway VPN",
                android.app.NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "DNS filtering is active"
            }
            val nm = getSystemService(android.app.NotificationManager::class.java)
            nm.createNotificationChannel(channel)
        }
    }

    override fun onStartCommand(intent: android.content.Intent?, flags: Int, startId: Int): Int {
        Log.i(TAG, "VpnService starting...")
        val notification = android.app.Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setContentTitle("Dnsway DNS Filter")
            .setContentText("Filtering DNS queries locally")
            .setSmallIcon(android.R.drawable.ic_menu_manage)
            .setOngoing(true)
            .build()
        startForeground(NOTIFICATION_ID, notification)
        startVpn()
        return START_STICKY
    }

    override fun onDestroy() {
        Log.i(TAG, "VpnService stopping...")
        isRunning = false
        tcpSessions.values.forEach { it.close() }
        tcpSessions.clear()
        synchronized(udpSockets) {
            udpSockets.values.forEach { it.close() }
            udpSockets.clear()
        }
        vpnThread?.join(3000)
        vpnInterface?.close()
        scope.cancel()
        Log.i(TAG, "VpnService stopped")
    }

    private fun startVpn() {
        val builder = Builder()
        builder.setMtu(1500)
        builder.addAddress(VPN_IP, 32)
        builder.addRoute("0.0.0.0", 0)          // all IPv4
        builder.addRoute("0:0:0:0:0:0:0:0", 0)  // all IPv6
        builder.addDnsServer(DNS_IP)

        vpnInterface = builder.establish() ?: run {
            Log.e(TAG, "Failed to establish VPN interface")
            return
        }

        val inputStream = FileInputStream(vpnInterface!!.fileDescriptor)
        val outputStream = FileOutputStream(vpnInterface!!.fileDescriptor)

        isRunning = true
        vpnThread = Thread {
            Log.i(TAG, "VPN thread started")
            try {
                val buf = ByteArray(4096)
                while (isRunning) {
                    val n = inputStream.read(buf)
                    if (n <= 0) continue
                    handlePacket(buf, n, outputStream)
                }
            } catch (e: Exception) {
                if (isRunning) Log.e(TAG, "VPN error: ${e.message}")
            } finally {
                Log.i(TAG, "VPN thread stopped")
            }
        }.apply { start() }
    }

    // ── Packet dispatch ───────────────────────────────────────────────

    private fun handlePacket(buf: ByteArray, len: Int, out: FileOutputStream) {
        when ((buf[0].toInt() shr 4) and 0x0f) {
            4 -> handleIpv4(buf, len, out)
            6 -> handleIpv6(buf, len, out)
        }
    }

    private fun handleIpv4(buf: ByteArray, len: Int, out: FileOutputStream) {
        val ihl = (buf[0].toInt() and 0x0f) * 4
        if (ihl < 20 || ihl > len) return
        when (buf[9].toInt() and 0xff) {
            6 -> handleTcp(buf, len, ihl, out)
            17 -> handleUdp(buf, len, ihl, out)
        }
        // ICMP(1), others → drop
    }

    private fun handleIpv6(buf: ByteArray, len: Int, out: FileOutputStream) {
        // Forward IPv6 as-is (no deep inspection).
        synchronized(out) { try { out.write(buf, 0, len) } catch (_: Exception) {} }
    }

    // ── UDP ───────────────────────────────────────────────────────────

    private fun handleUdp(buf: ByteArray, len: Int, ihl: Int, out: FileOutputStream) {
        val off = ihl
        if (off + 8 > len) return
        val dstPort = ((buf[off + 2].toInt() and 0xff) shl 8) or (buf[off + 3].toInt() and 0xff)
        val udpLen = ((buf[off + 4].toInt() and 0xff) shl 8) or (buf[off + 5].toInt() and 0xff)
        val dataOff = off + 8
        val dataLen = minOf(udpLen - 8, len - dataOff)
        if (dataLen <= 0) return

        when (dstPort) {
            53 -> handleDns(buf, len, ihl, off, dataOff, dataLen, out)
            853 -> {} // block DoT (UDP)
            else -> forwardUdp(buf, len, ihl, off, dataOff, dataLen, dstPort, out)
        }
    }

    private fun handleDns(buf: ByteArray, len: Int, ihl: Int, udpOff: Int,
                          dataOff: Int, dataLen: Int, out: FileOutputStream) {
        val dnsData = buf.copyOfRange(dataOff, dataOff + dataLen)
        try {
            val query = Message(dnsData)
            val question = query.getQuestion() ?: run { forwardUdpDns(dnsData, out, buf, len, ihl, udpOff, dataOff); return }
            val domain = question.getName().toString(true)
            if (domain.isBlank()) { forwardUdpDns(dnsData, out, buf, len, ihl, udpOff, dataOff); return }

            Log.d(TAG, "DNS: $domain")
            val decision = DnsEngine.processDomain(domain)
            Log.d(TAG, " => $decision")
            DnsEngine.recordQuery(domain, decision)

            if (decision == Decision.BLOCK) {
                writeUdpResponse(out, buf, ihl, udpOff, buildNxdomain(query))
            } else {
                forwardUdpDns(dnsData, out, buf, len, ihl, udpOff, dataOff)
            }
        } catch (e: Exception) {
            Log.w(TAG, "DNS parse error: ${e.message}")
            forwardUdpDns(dnsData, out, buf, len, ihl, udpOff, dataOff)
        }
    }

    private fun buildNxdomain(query: Message): ByteArray {
        val r = Message(query.getHeader().getID())
        r.getHeader().setFlag(Flags.QR.toInt())
        r.getHeader().setFlag(Flags.AA.toInt())
        r.getHeader().setFlag(Flags.RA.toInt())
        r.getHeader().setRcode(Rcode.NXDOMAIN)
        r.addRecord(query.getQuestion(), Section.QUESTION)
        return r.toWire()
    }

    private fun forwardUdpDns(data: ByteArray, out: FileOutputStream,
                              buf: ByteArray, len: Int, ihl: Int, udpOff: Int, dataOff: Int) {
        forwardUpstream(data, UPSTREAM_DNS, 53, out, buf, len, ihl, udpOff, dataOff)
    }

    private fun forwardUdp(buf: ByteArray, len: Int, ihl: Int, udpOff: Int,
                           dataOff: Int, dataLen: Int, dstPort: Int, out: FileOutputStream) {
        val dstIp = ipToString(buf, 16)
        val data = buf.copyOfRange(dataOff, dataOff + dataLen)
        forwardUpstream(data, dstIp, dstPort, out, buf, len, ihl, udpOff, dataOff)
    }

    private fun forwardUpstream(data: ByteArray, dstIp: String, dstPort: Int,
                                out: FileOutputStream, buf: ByteArray, len: Int,
                                ihl: Int, udpOff: Int, dataOff: Int) {
        val key = "$dstIp:$dstPort"
        val socket = synchronized(udpSockets) {
            udpSockets.getOrPut(key) { DatagramSocket().also { protect(it) } }
        }
        try {
            val pkt = DatagramPacket(data, data.size, InetAddress.getByName(dstIp), dstPort)
            socket.send(pkt)
            socket.soTimeout = 5000
            val resp = ByteArray(2048)
            val recv = DatagramPacket(resp, resp.size)
            socket.receive(recv)
            writeUdpResponse(out, buf, ihl, udpOff, recv.data.copyOf(recv.length))
        } catch (e: Exception) {
            synchronized(udpSockets) { udpSockets.remove(key) }
            socket.close()
        }
    }

    private fun writeUdpResponse(out: FileOutputStream, pkt: ByteArray,
                                 ihl: Int, udpOff: Int, data: ByteArray) {
        val dnsOff = udpOff + 8
        val newLen = dnsOff + data.size
        val np = ByteArray(newLen)
        System.arraycopy(pkt, 0, np, 0, dnsOff)
        for (i in 0..3) { np[12 + i] = pkt[16 + i]; np[16 + i] = pkt[12 + i] }
        np[udpOff] = pkt[udpOff + 2]; np[udpOff + 1] = pkt[udpOff + 3]
        np[udpOff + 2] = pkt[udpOff]; np[udpOff + 3] = pkt[udpOff + 1]
        System.arraycopy(data, 0, np, dnsOff, data.size)
        np[2] = ((newLen shr 8) and 0xff).toByte()
        np[3] = (newLen and 0xff).toByte()
        val ul = 8 + data.size
        np[udpOff + 4] = ((ul shr 8) and 0xff).toByte()
        np[udpOff + 5] = (ul and 0xff).toByte()
        np[udpOff + 6] = 0; np[udpOff + 7] = 0
        np[10] = 0; np[11] = 0
        val cs = ipChecksum(np, 0, ihl)
        np[10] = ((cs shr 8) and 0xff).toByte()
        np[11] = (cs and 0xff).toByte()
        synchronized(out) { out.write(np) }
    }

    private fun ipChecksum(data: ByteArray, off: Int, len: Int): Int {
        var sum = 0; var i = off
        while (i < off + len - 1) {
            sum += ((data[i].toInt() and 0xff) shl 8) or (data[i + 1].toInt() and 0xff)
            i += 2
        }
        if (i < off + len) sum += (data[i].toInt() and 0xff) shl 8
        while (sum > 0xffff) sum = (sum and 0xffff) + (sum shr 16)
        return sum.inv() and 0xffff
    }

    // ── DoH blocklist ─────────────────────────────────────────────────

    private val DOH_BLOCK_IPS = setOf(
        "1.1.1.1", "1.0.0.1",
        "8.8.8.8", "8.8.4.4",
        "9.9.9.9", "149.112.112.112",
        "208.67.222.222", "208.67.220.220",
        "94.140.14.14", "94.140.15.15",
        "185.228.168.9", "185.228.169.9",
        "76.76.19.19", "76.223.122.150",
    )

    // ── TCP ───────────────────────────────────────────────────────────

    /**
     * Intercept and relay TCP connections.
     *
     * For each new SYN we create a real [Socket] to the destination.
     * Data from the client (TUN) is stripped of headers and written to
     * the real socket. Data from the real server is wrapped in IP/TCP
     * headers and written back to the TUN.
     *
     * Connections to known DoH provider IPs on 443/853 are dropped.
     */
    private fun handleTcp(buf: ByteArray, len: Int, ihl: Int, out: FileOutputStream) {
        val tcpOff = ihl
        if (tcpOff + 20 > len) return
        val dstPort = ((buf[tcpOff + 2].toInt() and 0xff) shl 8) or (buf[tcpOff + 3].toInt() and 0xff)
        val srcPort = ((buf[tcpOff].toInt() and 0xff) shl 8) or (buf[tcpOff + 1].toInt() and 0xff)
        val flags = buf[tcpOff + 13].toInt() and 0x3f

        // Connection key = srcIp:srcPort
        val connKey = ipToLong(buf, 12, srcPort)

        // Block DoH/DoT to known provider IPs
        if ((dstPort == 443 || dstPort == 853) && isDohIp(buf, 16)) {
            Log.d(TAG, "Blocked connection to ${ipToString(buf, 16)}:$dstPort")
            sendRst(buf, ihl, tcpOff, out)
            return
        }

        // Don't intercept traffic to our own VPN DNS server
        if (dstPort == 53 && ipToString(buf, 16) == DNS_IP) return

        when {
            // SYN (no ACK) → new connection
            flags == 0x02 -> handleSyn(connKey, buf, len, ihl, tcpOff, dstPort, out)
            // FIN / RST → close
            flags and 0x01 != 0 || flags and 0x04 != 0 -> {
                tcpSessions.remove(connKey)?.close()
            }
            // Data or pure ACK
            else -> handleData(connKey, buf, len, ihl, tcpOff, flags, out)
        }
    }

    private fun isDohIp(buf: ByteArray, off: Int): Boolean {
        val ip = StringBuilder(15)
        ip.append(buf[off].toInt() and 0xff).append('.')
        ip.append(buf[off + 1].toInt() and 0xff).append('.')
        ip.append(buf[off + 2].toInt() and 0xff).append('.')
        ip.append(buf[off + 3].toInt() and 0xff)
        return ip.toString() in DOH_BLOCK_IPS
    }

    private fun handleSyn(connKey: Long, buf: ByteArray, len: Int, ihl: Int,
                          tcpOff: Int, dstPort: Int, out: FileOutputStream) {
        val dstIp = ipToString(buf, 16)
        val srcPort = ((buf[tcpOff].toInt() and 0xff) shl 8) or (buf[tcpOff + 1].toInt() and 0xff)
        val clientSeq = readU32(buf, tcpOff + 4)
        val id = nextTcpId++

        try {
            val socket = Socket()
            protect(socket)
            socket.connect(InetSocketAddress(dstIp, dstPort), 5000)

            val ourSeq = (System.currentTimeMillis() and 0xFFFFFFFF)
            val session = TcpSession(id, socket, buf.copyOfRange(12, 20), srcPort, dstPort,
                clientSeq, ourSeq, out, { tcpSessions.remove(connKey) })

            tcpSessions[connKey] = session

            writeTcpResponse(buf, ihl, tcpOff, ourSeq, clientSeq + 1, 0x12, null, out)

            // Start response reader
            Thread { session.responseLoop() }.apply {
                name = "tcp-$id"
                start()
            }
            Log.d(TAG, "TCP $dstIp:$dstPort connected (session=$id)")
        } catch (e: Exception) {
            Log.w(TAG, "TCP connect fail $dstIp:$dstPort: ${e.message}")
            sendRst(buf, ihl, tcpOff, out)
        }
    }

    private fun handleData(connKey: Long, buf: ByteArray, len: Int, ihl: Int,
                           tcpOff: Int, flags: Int, out: FileOutputStream) {
        val session = tcpSessions[connKey] ?: return
        val dataOff = tcpOff + ((buf[tcpOff + 12].toInt() and 0xf0) shr 2)
        val payloadLen = len - dataOff
        if (payloadLen <= 0) return

        try {
            val payload = buf.copyOfRange(dataOff, dataOff + payloadLen)
            session.socket.getOutputStream().write(payload)
            session.socket.getOutputStream().flush()
            session.clientBytesReceived.addAndGet(payloadLen.toLong())

            // ACK back to client
            writeTcpResponse(buf, ihl, tcpOff,
                session.ourSeq + 1 + session.serverBytesSent.get(),
                session.clientSeq + 1 + session.clientBytesReceived.get(),
                0x10, null, out)
        } catch (e: Exception) {
            Log.w(TAG, "TCP data error: ${e.message}")
            tcpSessions.remove(connKey)?.close()
        }
    }

    /** Send TCP RST to the peer. */
    private fun sendRst(buf: ByteArray, ihl: Int, tcpOff: Int, out: FileOutputStream) {
        writeTcpResponse(buf, ihl, tcpOff, 0, 0, 0x04, null, out)
    }

    /**
     * Build and write a TCP/IP response packet.
     * Always uses a clean 20-byte TCP header (no options) so the response
     * is independent of whatever TCP options the client sent.
     */
    private fun writeTcpResponse(orig: ByteArray, ihl: Int, tcpOff: Int,
                                 seq: Long, ack: Long, flags: Int,
                                 payload: ByteArray?,
                                 out: FileOutputStream) {
        val payLen = payload?.size ?: 0
        val tcpHdrLen = 20
        val newLen = ihl + tcpHdrLen + payLen
        val buf = ByteArray(newLen)

        // Copy IP header (will fix addresses and length)
        System.arraycopy(orig, 0, buf, 0, ihl)

        // Swap IP addresses
        for (i in 0..3) {
            buf[12 + i] = orig[16 + i]  // src = original dst (server)
            buf[16 + i] = orig[12 + i]  // dst = original src (client)
        }

        // IP total length
        buf[2] = ((newLen shr 8) and 0xff).toByte()
        buf[3] = (newLen and 0xff).toByte()

        // ── TCP header (20 bytes, no options) ──
        // Swap ports
        buf[ihl] = orig[tcpOff + 2]; buf[ihl + 1] = orig[tcpOff + 3]     // src = orig dst port
        buf[ihl + 2] = orig[tcpOff]; buf[ihl + 3] = orig[tcpOff + 1]     // dst = orig src port

        writeU32(buf, ihl + 4, seq)
        writeU32(buf, ihl + 8, ack)

        buf[ihl + 12] = 0x50              // data offset = 5 (20 bytes)
        buf[ihl + 13] = (flags and 0xff).toByte()  // flags
        buf[ihl + 14] = 0xFF.toByte()      // window
        buf[ihl + 15] = 0xFF.toByte()
        buf[ihl + 16] = 0; buf[ihl + 17] = 0  // checksum (computed)
        buf[ihl + 18] = 0; buf[ihl + 19] = 0  // urgent pointer

        if (payload != null) {
            System.arraycopy(payload, 0, buf, ihl + 20, payLen)
        }

        // IP checksum
        buf[10] = 0; buf[11] = 0
        val cs = ipChecksum(buf, 0, ihl)
        buf[10] = ((cs shr 8) and 0xff).toByte()
        buf[11] = (cs and 0xff).toByte()

        // TCP checksum
        val tcpLen = newLen - ihl
        val tcpCs = tcpChecksum(buf, ihl, tcpLen, buf.sliceArray(12..15), buf.sliceArray(16..19))
        buf[ihl + 16] = ((tcpCs shr 8) and 0xff).toByte()
        buf[ihl + 17] = (tcpCs and 0xff).toByte()

        synchronized(out) { out.write(buf) }
    }

    private fun tcpChecksum(data: ByteArray, off: Int, tcpLen: Int,
                            srcIp: ByteArray, dstIp: ByteArray): Int {
        var sum = 0L; var i = 0
        while (i < 4) {
            sum += ((srcIp[i].toInt() and 0xff) shl 8) or (dstIp[i].toInt() and 0xff)
            i++
        }
        sum += 6 // TCP protocol
        sum += tcpLen
        var j = off
        val end = off + tcpLen
        while (j < end - 1) {
            sum += ((data[j].toInt() and 0xff) shl 8) or (data[j + 1].toInt() and 0xff)
            j += 2
        }
        if (j < end) sum += (data[j].toInt() and 0xff) shl 8
        while (sum > 0xffff) sum = (sum and 0xffff) + (sum shr 16)
        return (sum.inv() and 0xffff).toInt()
    }

    // ── TCP Session ───────────────────────────────────────────────────

    /** Represents a proxied TCP connection. */
    inner class TcpSession(
        val id: Long,
        val socket: Socket,
        /** Original IP header (first 8 bytes: src IP, dst IP) */
        val ipHeader: ByteArray,
        val srcPort: Int,
        val dstPort: Int,
        /** Client's initial sequence number (from SYN) */
        val clientSeq: Long,
        /** Our chosen sequence number (sent in SYN-ACK) */
        val ourSeq: Long,
        val out: FileOutputStream,
        val onFinished: () -> Unit
    ) {
        val clientBytesReceived = java.util.concurrent.atomic.AtomicLong(0)
        val serverBytesSent = java.util.concurrent.atomic.AtomicLong(0)

        private val recvBuf = ByteArray(16384)
        @Volatile private var running = true

        fun responseLoop() {
            try {
                val sockIn = socket.getInputStream()
                while (running) {
                    val n = sockIn.read(recvBuf)
                    if (n < 0) break
                    if (n > 0) {
                        serverBytesSent.addAndGet(n.toLong())
                        val payload = recvBuf.copyOf(n)
                        val ack = clientSeq + 1 + clientBytesReceived.get()
                        val seq = ourSeq + 1 + (serverBytesSent.get() - n)

                        // Build a fresh IP/TCP response packet
                        val pkt = buildServerPacket(seq, ack, 0x18, payload)
                        synchronized(out) { out.write(pkt) }
                    }
                }
            } catch (_: Exception) {}
            finally {
                running = false
                onFinished()
                try { socket.close() } catch (_: Exception) {}
            }
        }

        /** Build a complete IP/TCP packet for data flowing server → client. */
        private fun buildServerPacket(seq: Long, ack: Long, flags: Int, payload: ByteArray): ByteArray {
            val payLen = payload.size
            val totalLen = 20 + 20 + payLen
            val buf = ByteArray(totalLen)

            // IP header
            buf[0] = 0x45; buf[1] = 0
            buf[2] = ((totalLen shr 8) and 0xff).toByte()
            buf[3] = (totalLen and 0xff).toByte()
            buf[4] = 0; buf[5] = 0
            buf[6] = 0x40; buf[7] = 0
            buf[8] = 64; buf[9] = 6
            buf[10] = 0; buf[11] = 0

            // src = server (original dst), dst = client (original src)
            for (i in 0..3) {
                buf[12 + i] = ipHeader[4 + i]  // server IP
                buf[16 + i] = ipHeader[i]       // client IP
            }

            // TCP header
            buf[20] = ((dstPort shr 8) and 0xff).toByte()
            buf[21] = (dstPort and 0xff).toByte()
            buf[22] = ((srcPort shr 8) and 0xff).toByte()
            buf[23] = (srcPort and 0xff).toByte()

            writeU32(buf, 24, seq)
            writeU32(buf, 28, ack)

            buf[32] = 0x50; buf[33] = (flags and 0xff).toByte()
            buf[34] = 0xFF.toByte(); buf[35] = 0xFF.toByte()
            buf[36] = 0; buf[37] = 0
            buf[38] = 0; buf[39] = 0

            System.arraycopy(payload, 0, buf, 40, payLen)

            // Checksums
            val ipCs = ipChecksum(buf, 0, 20)
            buf[10] = ((ipCs shr 8) and 0xff).toByte()
            buf[11] = (ipCs and 0xff).toByte()

            val tcpCs = tcpChecksum(buf, 20, 20 + payLen,
                buf.copyOfRange(12, 16), buf.copyOfRange(16, 20))
            buf[36] = ((tcpCs shr 8) and 0xff).toByte()
            buf[37] = (tcpCs and 0xff).toByte()

            return buf
        }

        fun close() {
            running = false
            try { socket.close() } catch (_: Exception) {}
        }
    }

    // ── Utilities ─────────────────────────────────────────────────────

    private fun readU32(buf: ByteArray, off: Int): Long {
        return ((buf[off].toInt() and 0xff).toLong() shl 24) or
               ((buf[off + 1].toInt() and 0xff).toLong() shl 16) or
               ((buf[off + 2].toInt() and 0xff).toLong() shl 8) or
               (buf[off + 3].toInt() and 0xff).toLong()
    }

    private fun writeU32(buf: ByteArray, off: Int, value: Long) {
        buf[off] = ((value shr 24) and 0xff).toByte()
        buf[off + 1] = ((value shr 16) and 0xff).toByte()
        buf[off + 2] = ((value shr 8) and 0xff).toByte()
        buf[off + 3] = (value and 0xff).toByte()
    }

    private fun ipToLong(buf: ByteArray, off: Int, port: Int): Long {
        return ((buf[off].toInt() and 0xff).toLong() shl 40) or
               ((buf[off + 1].toInt() and 0xff).toLong() shl 32) or
               ((buf[off + 2].toInt() and 0xff).toLong() shl 24) or
               ((buf[off + 3].toInt() and 0xff).toLong() shl 16) or
               (port.toLong() and 0xffffL)
    }

    private fun ipToString(buf: ByteArray, off: Int): String {
        return "${buf[off].toInt() and 0xff}.${buf[off + 1].toInt() and 0xff}." +
               "${buf[off + 2].toInt() and 0xff}.${buf[off + 3].toInt() and 0xff}"
    }

    companion object {
        const val TAG = "DnswayVpn"
        private const val NOTIFICATION_CHANNEL_ID = "dnsway_vpn"
        private const val NOTIFICATION_ID = 1
        @Volatile var isRunning = false
            private set

        fun start(context: android.content.Context) {
            val i = android.content.Intent(context, LocalDnsVpnService::class.java)
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O)
                context.startForegroundService(i)
            else
                context.startService(i)
        }

        fun stop(context: android.content.Context) {
            isRunning = false
            context.stopService(android.content.Intent(context, LocalDnsVpnService::class.java))
        }

        fun prepare(context: android.content.Context): android.content.Intent? {
            return VpnService.prepare(context)
        }
    }
}
