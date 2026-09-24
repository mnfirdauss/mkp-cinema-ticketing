#!/usr/bin/env python3
"""Render docs/diagrams/src/*.mmd (Mermaid) and *.html to JPG using headless Chrome + Mermaid.

Usage: python3 docs/diagrams/render.py   (requires google-chrome and Pillow)
"""
import html
import pathlib
import subprocess
import tempfile

from PIL import Image, ImageChops

Image.MAX_IMAGE_PIXELS = None  # screenshots are intentionally large

HERE = pathlib.Path(__file__).parent
TITLES = {
    "01-system-topology": ("System Topology", "Platform Tiket Bioskop Online – skala nasional, multi-cabang"),
    "02-flowchart-pembelian": ("Flowchart Pembelian Tiket", "Alur customer dari memilih film hingga menonton"),
    "03-flow-refund-pembatalan": ("Alur Pembatalan, Refund & Restok", "Ketika bioskop membatalkan jadwal tayang"),
    "04-erd": ("Entity Relationship Diagram", "Database PostgreSQL – db/schema.sql"),
}

PAGE = """<!doctype html><html><head><meta charset="utf-8">
<style>
  body {{ margin:0; background:#fff; font-family:'Segoe UI',Roboto,Helvetica,Arial,sans-serif; }}
  .wrap {{ display:inline-block; padding:40px 48px 36px; }}
  header {{ display:flex; align-items:center; gap:18px; margin-bottom:28px;
           border-bottom:4px solid #4F46E5; padding-bottom:16px; }}
  .logo {{ width:54px; height:54px; border-radius:50%;
          background:conic-gradient(#4F46E5,#7C3AED,#2563EB,#4F46E5); position:relative; }}
  .logo:after {{ content:''; position:absolute; inset:14px; background:#fff; border-radius:50%; }}
  h1 {{ margin:0; font-size:30px; color:#1E1B4B; }}
  p  {{ margin:4px 0 0; color:#475569; font-size:16px; }}
  .mermaid svg {{ max-width:none !important; }}
  g.start span, g.start p, g.start .nodeLabel {{ color:#fff !important; font-weight:600; }}
  footer {{ margin-top:22px; color:#94A3B8; font-size:13px; text-align:right; }}
</style></head><body><div class="wrap" id="wrap">
<header><div class="logo"></div><div><h1>{title}</h1><p>{subtitle}</p></div></header>
{body}
<footer>MKP Backend Development Test 2025 · Cinema Online Ticketing</footer>
</div>
<script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>
<script>
mermaid.initialize({{ startOnLoad:true, theme:'base', securityLevel:'loose',
  flowchart:{{ useMaxWidth:false, curve:'basis', htmlLabels:true, nodeSpacing:40, rankSpacing:55, padding:14 }},
  er:{{ useMaxWidth:false, layoutDirection:'TB', minEntityWidth:120 }},
  themeVariables:{{ fontSize:'16px', fontFamily:'Segoe UI, Roboto, Arial',
    primaryColor:'#EEF2FF', primaryBorderColor:'#6366F1', primaryTextColor:'#1E1B4B',
    lineColor:'#64748B', clusterBkg:'#F8FAFC', clusterBorder:'#CBD5E1' }} }});
</script></body></html>"""


def crop(img: Image.Image, margin: int = 24) -> Image.Image:
    bg = Image.new(img.mode, img.size, (255, 255, 255))
    bbox = ImageChops.difference(img, bg).getbbox()
    if not bbox:
        return img
    l, t, r, b = bbox
    return img.crop((max(l - margin, 0), max(t - margin, 0), min(r + margin, img.width), min(b + margin, img.height)))


def render(src: pathlib.Path) -> None:
    title, subtitle = TITLES.get(src.stem, (src.stem, ""))
    body = src.read_text() if src.suffix == ".html" else f'<pre class="mermaid">{html.escape(src.read_text())}</pre>'
    page = PAGE.format(title=title, subtitle=subtitle, body=body)
    with tempfile.TemporaryDirectory() as tmp:
        html_path = pathlib.Path(tmp) / "page.html"
        png_path = pathlib.Path(tmp) / "out.png"
        html_path.write_text(page)
        subprocess.run([
            "google-chrome", "--headless=new", "--disable-gpu", "--no-sandbox", "--hide-scrollbars",
            "--force-device-scale-factor=2", "--window-size=4200,7000", "--virtual-time-budget=15000",
            f"--screenshot={png_path}", html_path.as_uri(),
        ], check=True, capture_output=True)
        img = crop(Image.open(png_path).convert("RGB"))
    out = HERE / f"{src.stem}.jpg"
    img.save(out, "JPEG", quality=92, optimize=True)
    print(f"{out.relative_to(HERE.parent.parent)}  {img.width}x{img.height}")


if __name__ == "__main__":
    for f in sorted((HERE / "src").glob("*.*")):
        render(f)
