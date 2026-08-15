#!/usr/bin/env python3
import os
import json
import hmac
import hashlib
import base64
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

SECRET = os.environ.get("JWT_SECRET", "super-secret-key-12345")
PORT = int(os.environ.get("PORT", 8080))

ITEMS = [
    {"id": "item-1", "name": "Widget Alpha"},
    {"id": "item-2", "name": "Widget Beta"},
    {"id": "item-3", "name": "Gadget Gamma"},
    {"id": "item-4", "name": "Gadget Delta"},
    {"id": "item-5", "name": "Device Epsilon"},
]

def base64url_encode(data):
    if isinstance(data, str):
        data = data.encode("utf-8")
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("utf-8")

def base64url_decode(data):
    padding = 4 - len(data) % 4
    if padding != 4:
        data += "=" * padding
    return base64.urlsafe_b64decode(data)

def create_jwt(payload):
    header = base64url_encode(json.dumps({"alg": "HS256", "typ": "JWT"}))
    payload_b64 = base64url_encode(json.dumps(payload))
    message = f"{header}.{payload_b64}"
    signature = hmac.new(SECRET.encode(), message.encode(), hashlib.sha256).digest()
    signature_b64 = base64url_encode(signature)
    return f"{message}.{signature_b64}"

def verify_jwt(token):
    try:
        parts = token.split(".")
        if len(parts) != 3:
            return None
        header_b64, payload_b64, signature_b64 = parts
        message = f"{header_b64}.{payload_b64}"
        expected_sig = hmac.new(SECRET.encode(), message.encode(), hashlib.sha256).digest()
        expected_sig_b64 = base64url_encode(expected_sig)
        if signature_b64 != expected_sig_b64:
            return None
        payload = json.loads(base64url_decode(payload_b64))
        return payload
    except Exception:
        return None

def create_cursor(offset):
    return base64url_encode(json.dumps({"offset": offset}))

def decode_cursor(cursor):
    try:
        data = json.loads(base64url_decode(cursor))
        return data.get("offset")
    except Exception:
        return None

class RequestHandler(BaseHTTPRequestHandler):
    def send_json(self, status, data, headers=None):
        body = json.dumps(data).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", len(body))
        if headers:
            for k, v in headers.items():
                self.send_header(k, v)
        self.end_headers()
        self.wfile.write(body)

    def send_error_json(self, status, message):
        self.send_json(status, {"error": message})

    def do_POST(self):
        if self.path == "/login":
            self.handle_login()
        else:
            self.send_error_json(404, "Not found")

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)

        if path == "/api/items":
            self.handle_get_items(query)
        elif path.startswith("/api/items/"):
            item_id = path[len("/api/items/"):]
            self.handle_get_item(item_id)
        else:
            self.send_error_json(404, "Not found")

    def handle_login(self):
        try:
            content_length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(content_length)
            data = json.loads(body)
        except (json.JSONDecodeError, ValueError):
            self.send_error_json(400, "Invalid JSON")
            return

        user = data.get("user")
        password = data.get("pass")

        if user == "admin" and password == "secret":
            payload = {"user": user, "iat": int(time.time())}
            token = create_jwt(payload)
            self.send_json(200, {"token": token})
        else:
            self.send_error_json(401, "Invalid credentials")

    def get_auth_token(self):
        auth = self.headers.get("Authorization", "")
        if auth.startswith("Bearer "):
            return auth[7:]
        return None

    def require_auth(self):
        token = self.get_auth_token()
        if not token:
            self.send_error_json(401, "Missing authorization token")
            return None
        payload = verify_jwt(token)
        if not payload:
            self.send_error_json(401, "Invalid or expired token")
            return None
        return payload

    def handle_get_items(self, query):
        if not self.require_auth():
            return

        try:
            limit = int(query.get("limit", ["10"])[0])
        except ValueError:
            self.send_error_json(400, "Invalid limit")
            return

        cursor_param = query.get("cursor", [None])[0]
        offset = 0
        if cursor_param:
            offset = decode_cursor(cursor_param)
            if offset is None:
                self.send_error_json(400, "Invalid cursor")
                return

        end = offset + limit
        page_items = ITEMS[offset:end]
        next_offset = end

        next_cursor = None
        if next_offset < len(ITEMS):
            next_cursor = create_cursor(next_offset)

        self.send_json(200, {"items": page_items, "next_cursor": next_cursor})

    def handle_get_item(self, item_id):
        if not self.require_auth():
            return

        item = None
        for i in ITEMS:
            if i["id"] == item_id:
                item = i
                break

        if not item:
            self.send_error_json(404, "Item not found")
            return

        etag = f'"{base64url_encode(item_id)}"'
        if_none_match = self.headers.get("If-None-Match")
        if if_none_match == etag:
            self.send_response(304)
            self.send_header("ETag", etag)
            self.end_headers()
            return

        self.send_json(200, item, {"ETag": etag})

    def log_message(self, format, *args):
        pass

if __name__ == "__main__":
    server = HTTPServer(("0.0.0.0", PORT), RequestHandler)
    print(f"Server running on port {PORT}")
    server.serve_forever()
