import ipaddress
from array import array
from enum import Enum


class TLSSession:
    client_ip: str
    user: str
    sni: str
    server_addr: str
    source_ip: str
    acls: set
    scores: map[str, int]
    action: str
    possible_actions: tuple


class Method(Enum):
    GET = "GET"
    POST = "POST"
    CONNECT = "CONNECT"
    OPTIONS = "OPTIONS"


class Request:
    client_ip: str
    user: str
    method: Method
    url: str
    host: str
    path: str
    header: map[str, str]
    query: map[str, str]
    acls: str
    scores: str
    action: str
    possible_actions: str


class Response:
    request: Request
    status: int
    body: str
    header: map[str, str]
    query: map[str, str]
    acls: str
    scores: str
    action: str
    possible_actions: str

    def thumbnail(self, size: int) -> str:
        return str(size)


class json:
    def decode(x: str) -> any:
        return None

    def encode(x: any) -> str:
        return str(any)