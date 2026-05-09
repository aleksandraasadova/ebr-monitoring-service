"""Generate architecture diagrams for iObserve EBR README."""
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch

# ── Palette ────────────────────────────────────────────────────────
PRIMARY  = '#12205c'
NAVY     = '#1c2b3a'
GOLD     = '#f5c04f'
BG       = '#f7f9fc'
BORDER   = '#c4d4e8'
WHITE    = '#ffffff'
MUTED    = '#6b7b8d'
OK       = '#0f6e34'
WARN     = '#8a5600'
ERR      = '#8b1c1c'
LIGHT_OK = '#e3f5eb'
LIGHT_W  = '#fef6e0'
LIGHT_B  = '#dbeafe'


def box(ax, x, y, w, h, label, sublabel='', color=PRIMARY, textcolor=WHITE,
        subcolor=BORDER, fontsize=10, subsize=8, radius=0):
    rect = FancyBboxPatch((x, y), w, h,
                          boxstyle=f'round,pad=0' if radius == 0 else f'round,pad={radius}',
                          linewidth=1.4, edgecolor=color,
                          facecolor=color)
    ax.add_patch(rect)
    ty = y + h / 2 + (0.08 if sublabel else 0)
    ax.text(x + w / 2, ty, label,
            ha='center', va='center', fontsize=fontsize,
            color=textcolor, fontweight='bold', fontfamily='monospace')
    if sublabel:
        ax.text(x + w / 2, y + h / 2 - 0.12, sublabel,
                ha='center', va='center', fontsize=subsize,
                color=textcolor, alpha=0.75, fontfamily='sans-serif')


def light_box(ax, x, y, w, h, label, sublabel='', facecolor=LIGHT_B,
              edgecolor=PRIMARY, fontsize=9, subsize=7.5):
    rect = FancyBboxPatch((x, y), w, h,
                          boxstyle='round,pad=0',
                          linewidth=1.2, edgecolor=edgecolor,
                          facecolor=facecolor)
    ax.add_patch(rect)
    ty = y + h / 2 + (0.07 if sublabel else 0)
    ax.text(x + w / 2, ty, label,
            ha='center', va='center', fontsize=fontsize,
            color=edgecolor, fontweight='bold')
    if sublabel:
        ax.text(x + w / 2, y + h / 2 - 0.1, sublabel,
                ha='center', va='center', fontsize=subsize,
                color=MUTED)


def arrow(ax, x1, y1, x2, y2, label='', color=PRIMARY, lw=1.5):
    ax.annotate('', xy=(x2, y2), xytext=(x1, y1),
                arrowprops=dict(arrowstyle='->', color=color,
                                lw=lw, mutation_scale=14))
    if label:
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        ax.text(mx + 0.04, my, label, fontsize=7.5, color=color,
                va='center', fontfamily='monospace')


# ══════════════════════════════════════════════════════════════════
#  DIAGRAM 1 — System Architecture
# ══════════════════════════════════════════════════════════════════
fig, ax = plt.subplots(figsize=(14, 8))
fig.patch.set_facecolor(BG)
ax.set_facecolor(BG)
ax.set_xlim(0, 14)
ax.set_ylim(0, 8)
ax.axis('off')

ax.text(7, 7.6, 'iObserve EBR — System Architecture',
        ha='center', va='center', fontsize=14, fontweight='bold',
        color=PRIMARY, fontfamily='sans-serif')

# ── Browser client ────────────────────────────────────────────────
box(ax, 0.3, 5.5, 2.2, 1.0, 'Browser / Client',
    'Operator · Admin', color=NAVY, fontsize=9)

# ── EBR Service ───────────────────────────────────────────────────
ebr_x, ebr_y, ebr_w, ebr_h = 4.5, 1.8, 5.0, 5.0
outer = FancyBboxPatch((ebr_x, ebr_y), ebr_w, ebr_h,
                       boxstyle='round,pad=0',
                       linewidth=2, edgecolor=PRIMARY,
                       facecolor=WHITE)
ax.add_patch(outer)
ax.text(ebr_x + ebr_w / 2, ebr_y + ebr_h - 0.28,
        'EBR Service  (Go, net/http)',
        ha='center', va='center', fontsize=10.5,
        fontweight='bold', color=PRIMARY)

# inner layers
layer_data = [
    (4.7, 5.8, 4.6, 0.75, 'Transport Layer',
     'HTTP Handlers · JWT MW · RBAC · WebSocket · Swagger UI', LIGHT_B, PRIMARY),
    (4.7, 4.7, 4.6, 0.75, 'Service Layer',
     'AuthService · BatchService · RecipeService · UserService', LIGHT_OK, OK),
    (4.7, 3.6, 4.6, 0.75, 'Repository Layer',
     'UserRepo · BatchRepo · RecipeRepo', LIGHT_W, WARN),
    (4.7, 2.5, 4.6, 0.75, 'Domain (Core)',
     'Entities · Interfaces · Sentinel Errors · DTOs', '#fde8e8', ERR),
]
for lx, ly, lw, lh, lb, ls, fc, ec in layer_data:
    light_box(ax, lx, ly, lw, lh, lb, ls, facecolor=fc, edgecolor=ec,
              fontsize=9, subsize=7.5)

# ── PostgreSQL ────────────────────────────────────────────────────
box(ax, 10.8, 3.5, 2.6, 1.2, 'PostgreSQL 18',
    '6 migrations\nport 5433', color=NAVY, fontsize=9)

# ── MQTT Mosquitto ────────────────────────────────────────────────
box(ax, 0.3, 2.8, 2.2, 1.0, 'MQTT Broker',
    'Mosquitto 2.0\nport 1883', color=WARN, textcolor=WHITE, fontsize=9)

# ── PLC Simulator ────────────────────────────────────────────────
box(ax, 0.3, 1.2, 2.2, 1.0, 'PLC Simulator',
    'production-service\n(mock)', color=OK, fontsize=9)

# ── golang-migrate ────────────────────────────────────────────────
box(ax, 10.8, 1.5, 2.6, 1.0, 'golang-migrate',
    'v4  ·  up / down', color='#334155', fontsize=9)

# ── Arrows ────────────────────────────────────────────────────────
# Browser -> EBR
arrow(ax, 2.5, 6.0, 4.5, 5.5, 'HTTP REST / JSON', color=PRIMARY, lw=1.8)
# EBR -> Browser (websocket)
arrow(ax, 4.5, 5.3, 2.5, 5.7, 'WebSocket', color=PRIMARY, lw=1.4)

# PLC -> MQTT
arrow(ax, 1.4, 2.2, 1.4, 2.8, 'publish', color=WARN, lw=1.4)
# MQTT -> EBR
arrow(ax, 2.5, 3.3, 4.5, 3.8, 'subscribe', color=WARN, lw=1.4)

# EBR -> Postgres
arrow(ax, 9.5, 4.0, 10.8, 4.0, 'SQL / pq', color=NAVY, lw=1.8)

# migrate -> postgres
arrow(ax, 12.1, 2.5, 12.1, 3.5, 'run migrations', color='#334155', lw=1.2)

# legend
leg_items = [
    mpatches.Patch(facecolor=PRIMARY, label='Core service'),
    mpatches.Patch(facecolor=OK,      label='PLC / Equipment'),
    mpatches.Patch(facecolor=WARN,    label='MQTT broker'),
    mpatches.Patch(facecolor=NAVY,    label='Database'),
]
ax.legend(handles=leg_items, loc='lower right', fontsize=8,
          framealpha=0.9, edgecolor=BORDER)

plt.tight_layout()
plt.savefig('docs/arch-system.png', dpi=150, bbox_inches='tight',
            facecolor=BG)
plt.close()
print('arch-system.png done')


# ══════════════════════════════════════════════════════════════════
#  DIAGRAM 2 — Layered Architecture (3-layer)
# ══════════════════════════════════════════════════════════════════
fig2, ax2 = plt.subplots(figsize=(12, 9))
fig2.patch.set_facecolor(BG)
ax2.set_facecolor(BG)
ax2.set_xlim(0, 12)
ax2.set_ylim(0, 9)
ax2.axis('off')

ax2.text(6, 8.6, 'iObserve EBR — Layered Architecture (Go)',
         ha='center', va='center', fontsize=14,
         fontweight='bold', color=PRIMARY)

layers = [
    # (y_top, color, edge, title, items_left, items_right)
    (7.2, LIGHT_B,  PRIMARY, 'Transport Layer',
     ['POST /api/v1/auth/login', 'POST /api/v1/users', 'GET  /api/v1/recipes/{code}', 'GET  /swagger/  (OpenAPI UI)'],
     ['POST/GET /api/v1/batches', 'JWT MW → typed Claims in ctx', 'RBAC RequireRole(...)']),
    (5.3, LIGHT_OK, OK,     'Service Layer',
     ['AuthService — login, bcrypt, JWT issue', 'UserService — create, translit username'],
     ['RecipeService — get by code, archive check', 'BatchService — validate volume, no DB / no tx']),
    (3.4, LIGHT_W,  WARN,   'Repository Layer',
     ['UserRepo — find by username, create'],
     ['RecipeRepo — get by code', 'BatchRepo — create batch + weighing_log (owns tx)']),
    (1.5, '#fde8e8', ERR,   'Domain (Core — no deps)',
     ['Batch, Recipe, User — entities', 'BatchRepo, RecipeRepo, UserRepo — interfaces'],
     ['ErrRecipeNotFound, ErrRecipeArchived', 'ErrInvalidBatchVolume, ErrNoUserFound', 'Request/Response DTOs']),
]

for (ly, fc, ec, title, left_items, right_items) in layers:
    # background panel
    panel = FancyBboxPatch((0.4, ly - 1.5), 11.2, 1.7,
                           boxstyle='round,pad=0',
                           linewidth=1.5, edgecolor=ec, facecolor=fc)
    ax2.add_patch(panel)
    # title bar
    title_bar = FancyBboxPatch((0.4, ly - 0.02), 11.2, 0.38,
                               boxstyle='round,pad=0',
                               linewidth=0, edgecolor=ec, facecolor=ec)
    ax2.add_patch(title_bar)
    ax2.text(6, ly + 0.17, title,
             ha='center', va='center', fontsize=11,
             fontweight='bold', color=WHITE)
    # left items
    for i, item in enumerate(left_items):
        ax2.text(0.7, ly - 0.35 - i * 0.32, f'• {item}',
                 va='center', fontsize=8, color=ec,
                 fontfamily='monospace')
    # right items
    for i, item in enumerate(right_items):
        ax2.text(6.2, ly - 0.35 - i * 0.32, f'• {item}',
                 va='center', fontsize=8, color=ec,
                 fontfamily='monospace')

# arrows between layers
for ay_pos in [5.7, 3.8, 1.9]:
    ax2.annotate('', xy=(6, ay_pos - 0.25), xytext=(6, ay_pos),
                 arrowprops=dict(arrowstyle='->', color=MUTED,
                                 lw=1.5, mutation_scale=14))

# HTTP request at top
box(ax2, 4.5, 8.0, 3.0, 0.48, 'HTTP Request', color=NAVY, fontsize=9)
arrow(ax2, 6.0, 8.0, 6.0, 7.58, color=PRIMARY, lw=1.5)

# PostgreSQL at bottom
box(ax2, 4.5, 0.15, 3.0, 0.48, 'PostgreSQL 18', color=NAVY, fontsize=9)
arrow(ax2, 6.0, 1.5, 6.0, 0.63, color=NAVY, lw=1.5)

plt.tight_layout()
plt.savefig('docs/arch-layers.png', dpi=150, bbox_inches='tight',
            facecolor=BG)
plt.close()
print('arch-layers.png done')


# ══════════════════════════════════════════════════════════════════
#  DIAGRAM 3 — Request flow: Register Batch
# ══════════════════════════════════════════════════════════════════
fig3, ax3 = plt.subplots(figsize=(13, 6))
fig3.patch.set_facecolor(BG)
ax3.set_facecolor(BG)
ax3.set_xlim(0, 13)
ax3.set_ylim(0, 6)
ax3.axis('off')

ax3.text(6.5, 5.65, 'Request Flow — POST /api/v1/batches',
         ha='center', va='center', fontsize=13,
         fontweight='bold', color=PRIMARY)

actors = [
    (0.8,  'Client\n(Browser)', NAVY),
    (2.7,  'JWT\nMiddleware',   PRIMARY),
    (4.6,  'Batch\nHandler',    PRIMARY),
    (6.5,  'Batch\nService',    OK),
    (8.4,  'Recipe\nRepo',      WARN),
    (10.3, 'Batch\nRepo',       WARN),
    (12.0, 'PostgreSQL',        NAVY),
]

# actor boxes
for ax_x, alabel, acolor in actors:
    box(ax3, ax_x - 0.7, 4.4, 1.4, 0.7, alabel,
        color=acolor, fontsize=8)
    # lifeline
    ax3.plot([ax_x, ax_x], [4.4, 0.4],
             color=acolor, lw=0.8, linestyle='--', alpha=0.4)

steps = [
    # (from_x, to_x, y, label, color, return)
    (0.8,  2.7,  4.05, 'POST /api/v1/batches\n+ Bearer token', PRIMARY, False),
    (2.7,  4.6,  3.70, 'JWT → Claims{UserID,Role}\nin ctx (typed)', PRIMARY, False),
    (4.6,  6.5,  3.40, 'CreateBatch(ctx, req,\nclaims.UserID)', OK, False),
    (6.5,  8.4,  3.10, 'GetByCode(code)', WARN, False),
    (8.4,  12.0, 2.80, 'SELECT * FROM recipes', NAVY, False),
    (12.0, 8.4,  2.50, '→ Recipe', NAVY, True),
    (8.4,  6.5,  2.20, '→ *Recipe', WARN, True),
    (6.5,  6.5,  1.95, 'validate volume\n(no DB, no tx)', OK, False),
    (6.5,  10.3, 1.65, 'Create(batch, recipeID)', WARN, False),
    (10.3, 12.0, 1.35, 'BEGIN · INSERT batch ·\nINSERT weighing_log · COMMIT', NAVY, False),
    (12.0, 10.3, 1.05, '→ ok', NAVY, True),
    (10.3, 6.5,  0.80, '→ nil', WARN, True),
    (6.5,  4.6,  0.55, '→ Response', OK, True),
    (4.6,  0.8,  0.30, '201 Created', PRIMARY, True),
]

for fx, tx, sy, slabel, scolor, ret in steps:
    style = '<-' if ret else '->'
    ax3.annotate('', xy=(tx, sy), xytext=(fx, sy),
                 arrowprops=dict(arrowstyle=style,
                                 color=scolor, lw=1.3,
                                 mutation_scale=12,
                                 linestyle='dashed' if ret else 'solid'))
    mx = (fx + tx) / 2
    dy = 0.12 if '\n' not in slabel else 0.16
    ax3.text(mx, sy + dy, slabel,
             ha='center', va='bottom', fontsize=7,
             color=scolor, fontfamily='monospace')

plt.tight_layout()
plt.savefig('docs/arch-flow.png', dpi=150, bbox_inches='tight',
            facecolor=BG)
plt.close()
print('arch-flow.png done')
