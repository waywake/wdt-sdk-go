# -*- coding: utf-8 -*-
"""抓取旺店通旗舰版开放平台 API 文档页,提取接口元数据。

输出 scripts/data/apis.json,原始 HTML 缓存到 scripts/data/pages/ 以便离线重析。
"""
import json
import os
import re
import sys
import time
import html as H
import urllib.request
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.join(HERE, "data")
PAGES = os.path.join(DATA, "pages")
DOC_URL = "https://open.wangdian.cn/qjb/open/apidoc/doc?path={method}"


def load_methods():
    methods = []
    with open(os.path.join(DATA, "methods.txt"), encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            parts = line.split("\t")
            methods.append({"method": parts[0], "category": parts[1], "name": parts[2] if len(parts) > 2 else ""})
    return methods


def fetch(method):
    """下载文档页,优先使用本地缓存。"""
    safe = method.replace("/", "_")
    cache = os.path.join(PAGES, safe + ".html")
    if os.path.exists(cache) and os.path.getsize(cache) > 20000:
        with open(cache, encoding="utf-8") as f:
            return f.read()
    url = DOC_URL.format(method=urllib.parse.quote(method))
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0 (wdt-sdk-go scraper)"})
    last_err = None
    for attempt in range(3):
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                body = resp.read().decode("utf-8", "replace")
            if len(body) < 10000:
                raise RuntimeError("page too small: %d" % len(body))
            with open(cache, "w", encoding="utf-8") as f:
                f.write(body)
            return body
        except Exception as e:  # noqa: BLE001
            last_err = e
            time.sleep(1.5 * (attempt + 1))
    raise RuntimeError("fetch %s failed: %s" % (method, last_err))


def strip_tags(s):
    t = re.sub(r"<[^>]+>", "", s)
    t = H.unescape(t)
    t = t.replace("\u00a0", " ")
    return re.sub(r"\s+", " ", t).strip()


def json_compact(s):
    """压缩示例 JSON 便于对比(仅去空白)。"""
    return re.sub(r"\s+", "", s) if s else s


def table_rows(tb):
    rows = []
    for tr in re.findall(r"<tr.*?</tr>", tb, re.S):
        cells = [strip_tags(td) for td in re.findall(r"<td.*?</td>", tr, re.S)]
        if cells:
            rows.append(cells)
    return rows


HEAD_RE = re.compile(r'<a name="([^"]+)"></a>\s*<strong[^>]*>([^<]{2,60})</strong>|<(?:strong|b)[^>]*>(\d(?:\.\d)*\s*[^<]{1,50})</(?:strong|b)>')


NUM_TITLE = re.compile(r"^(\d+(?:\.\d+)*)[\s.、]*(.*)$")


def sections_with_tables(raw):
    """按文档顺序返回 [(最近的编号标题, table_rows)]。

    标题来源: <a name="x.y 标题"> 锚点或加粗的编号标题。
    """
    events = []
    for m in re.finditer(r"<table.*?</table>", raw, re.S):
        events.append(("table", m.start(), m.group(0)))
    for m in re.finditer(r'<a name="([^"]+)"></a>\s*(?:<strong[^>]*>)?([^<]{1,80})', raw):
        text = strip_tags(m.group(2))
        if NUM_TITLE.match(m.group(1).strip()) or (text and NUM_TITLE.match(text)):
            events.append(("head", m.start(), text or m.group(1)))
    for m in re.finditer(r"<strong[^>]*>\s*(\d(?:\.\d+){0,3}[\s.、][^<]{1,60})</strong>", raw):
        events.append(("head", m.start(), strip_tags(m.group(1))))
    events.sort(key=lambda e: e[1])
    out, cur_head = [], ""
    for kind, _, payload in events:
        if kind == "head":
            cur_head = payload
        else:
            rows = table_rows(payload)
            if rows:
                out.append((cur_head, rows))
    return out


def section_level(head):
    """'3.2 业务请求参数' -> (3, 2);无编号返回 (None, None)。"""
    m = NUM_TITLE.match(head or "")
    if not m:
        return None, None
    parts = m.group(1).split(".")
    try:
        return int(parts[0]), int(parts[1]) if len(parts) > 1 else None
    except ValueError:
        return None, None


SNAKE = re.compile(r"^[a-z][a-z0-9_]*$")

# 字段表里的字段名(支持 snake_case 和 camelCase)
IDENT = re.compile(r"^[a-z][A-Za-z0-9_]*$")

# SDK 函数签名表里出现的参数类型名
TYPE_NAMES = {
    "String", "Int", "Integer", "Long", "long", "Double", "Float", "Boolean", "Bool",
    "Object", "Pager", "Map", "List", "List<String>", "Map<String, Object>",
    "Map<String,Object>", "List<Map<String, Object>>", "List<Map<String,Object>>",
    "List<Object>", "String[]", "int", "date", "Date", "datetime",
}


def norm_type(t):
    """规范化类型字符串,便于比较。"""
    t = (t or "").strip().replace("＜", "<").replace("＞", ">")
    return re.sub(r"\s+", "", t)


def parse_page(raw):
    info = {"meta": [], "desc": "", "sdk_sig": [], "req_sections": [], "resp_sections": [],
            "req_example": "", "resp_example": "", "err_example": "", "php_example": ""}
    examples = []
    zone = "req"  # 请求区 / 响应区
    for head, rows in sections_with_tables(raw):
        flat_first = rows[0][0] if rows and rows[0] else ""
        sec, sub = section_level(head)
        if sec is not None and sec >= 4:
            zone = "resp"
        # 接口说明表
        if flat_first.startswith("1.1") or "接口描述" in flat_first:
            for r in rows:
                if r:
                    info["meta"].append(r[0])
                    m = re.match(r"1\.1\s*接口描述[：:](.*)", r[0])
                    if m:
                        info["desc"] = m.group(1).strip()
            continue
        # 示例表(JSON/PHP/JAVA/C#/CURL/json格式请求报文 行)
        first_label = (rows[0][0] or "").lower()
        if rows[0] and (rows[0][0] in ("JSON", "PHP", "JAVA", "C#", "CURL") or "json" in first_label):
            for r in rows:
                if len(r) >= 2:
                    label = r[0].lower()
                    if label == "json" or "json格式" in label or "json请求" in label:
                        examples.append(r[1])
                    elif label == "php" and not info["php_example"]:
                        info["php_example"] = r[1]
                    elif label == "curl" and not info.get("curl_example"):
                        info["curl_example"] = r[1]
            continue
        # 公共请求参数表(所有接口相同,跳过)
        col1 = [r[1] if len(r) > 1 else "" for r in rows]
        if "sid" in col1 and "salt" in col1:
            continue
        # 字段表: 第2列为标识符字段名
        field_rows = []
        for r in rows:
            if len(r) >= 4 and r[1] and IDENT.match(r[1]):
                field_rows.append({
                    "cn": r[0],
                    "field": r[1],
                    "type": r[2] if len(r) > 2 else "",
                    "length": r[3] if len(r) > 3 else "",
                    "required": (r[4] if len(r) > 4 else ""),
                    "desc": (r[5] if len(r) > 5 else (r[4] if len(r) > 4 else "")),
                })
        if not field_rows:
            continue
        # 公共响应参数表(status/message/data 及其变体,含文档typo 'messge') → 进入响应区
        names = [f["field"] for f in field_rows]
        if names and names[0] == "status" and ({"message", "messge", "data"} & set(names[1:3])):
            zone = "resp"
        # SDK 函数签名表(仅请求区;少量行且含复合/分页类型)
        if zone == "req" and not info["sdk_sig"] and len(field_rows) <= 8 \
                and any("<" in f["type"] or norm_type(f["type"]).lower() in ("pager", "object", "map", "list") for f in field_rows):
            info["sdk_sig"] = [{"name": f["field"], "cn": f["cn"], "type": f["type"], "required": f["required"]} for f in field_rows]
            continue
        if zone == "req":
            info["req_sections"].append({"heading": head, "fields": field_rows})
        else:
            info["resp_sections"].append({"heading": head, "fields": field_rows})
    # 分类示例: 信封 {status,...} 为响应,其余数组为请求体
    for ex in examples:
        s = strip_json_comments(ex).strip()
        if not s:
            continue
        if s.startswith("{"):
            try:
                obj = json.loads(s)
                status = obj.get("status")
                if status in (0, "0"):
                    info["resp_example"] = ex
                else:
                    info["err_example"] = ex
            except ValueError:
                pass
        elif s.startswith("[") and not info["req_example"]:
            info["req_example"] = s
    return info


def strip_json_comments(s):
    """去掉示例 JSON 中的 // 行注释（保留字符串内的 //）。"""
    out = []
    in_str = False
    esc = False
    i = 0
    while i < len(s):
        c = s[i]
        if in_str:
            out.append(c)
            if esc:
                esc = False
            elif c == "\\":
                esc = True
            elif c == '"':
                in_str = False
            i += 1
            continue
        if c == '"':
            in_str = True
            out.append(c)
            i += 1
            continue
        if c == "/" and i + 1 < len(s) and s[i + 1] == "/":
            # 注释持续到行尾；若行尾前出现闭括号则停在那里（文档示例常见 //备注 } ]）
            while i < len(s) and s[i] not in "\n}]":
                i += 1
            continue
        out.append(c)
        i += 1
    return "".join(out)


def main():
    os.makedirs(PAGES, exist_ok=True)
    methods = load_methods()
    result, failed = [], []
    for i, m in enumerate(methods):
        try:
            raw = fetch(m["method"])
        except Exception as e:  # noqa: BLE001
            failed.append({"method": m["method"], "error": str(e)})
            print("[%d/%d] FAIL %s: %s" % (i + 1, len(methods), m["method"], e), file=sys.stderr)
            continue
        info = parse_page(raw)
        rec = dict(m)
        rec.update(info)
        result.append(rec)
        nfields = sum(len(s["fields"]) for s in info["req_sections"]) + sum(len(s["fields"]) for s in info["resp_sections"])
        print("[%d/%d] %s (%s) req=%d resp=%d ex=%s" % (
            i + 1, len(methods), m["method"], m["name"],
            sum(len(s["fields"]) for s in info["req_sections"]),
            sum(len(s["fields"]) for s in info["resp_sections"]),
            "Y" if info["req_example"] else "n"))
        time.sleep(0.25)
    with open(os.path.join(DATA, "apis.json"), "w", encoding="utf-8") as f:
        json.dump({"apis": result, "failed": failed}, f, ensure_ascii=False, indent=1)
    print("done. ok=%d failed=%d" % (len(result), len(failed)))


if __name__ == "__main__":
    main()
