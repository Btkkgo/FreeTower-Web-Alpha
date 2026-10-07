import { publicCharacterName } from "../../identity/model";
import { identityAPI } from "../../identity/api";
import { recoverUnauthorized, reconnectSession } from "../../identity/recovery";
import LifeHUD, { useLife } from "./life/LifeHUD";
import BuildingHUD, { useBuilding } from "./building/BuildingHUD";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { api } from "../../game/api";
import type {
  Player,
  Room,
  RoomDetail,
  ServerMessage,
  User,
} from "../../game/types";
import FreeTowerWorld, { type Motion, type Readout } from "./World";
import { locomotionAt, movePayload, worldConfig } from "./model";
import "./freetower.css";
import { playGatherHit } from "./gathering/audio";
import GatheringHUD, { useGathering } from "./gathering/GatheringHUD";
import { definitions, config as gatheringConfig } from "./gathering/model";
const initialMotion = (): Motion => ({
  position: { ...worldConfig.spawn },
  facing: 0,
  moving: false,
  running: false,
  jumpAt: 0,
  ...locomotionAt(worldConfig.spawn),
});
type Chat = Extract<ServerMessage, { type: "chat" }>;
export default function FreeTowerApp({ identityUser, onIdentityDisconnect, onIdentityRevoked }: { identityUser?: User; onIdentityDisconnect?: () => Promise<void>; onIdentityRevoked?: () => void } = {}) {
  const [user, setUser] = useState<User | null>(identityUser ?? null),
    [checking, setChecking] = useState(true),
    [rooms, setRooms] = useState<Room[]>([]),
    [detail, setDetail] = useState<RoomDetail | null>(null),
    [players, setPlayers] = useState<Player[]>([]),
    [connected, setConnected] = useState(false),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [register, setRegister] = useState(false),
    [settings, setSettings] = useState(false),
    [chat, setChat] = useState<Chat[]>([]),
    [readout, setReadout] = useState<Readout | null>(null);
  const motion = useRef<Motion>(initialMotion()),
    ws = useRef<WebSocket | null>(null),
    debug = new URLSearchParams(location.search).get("debug") === "1";
  const gathering = useGathering(
    (message) => {
      if (ws.current?.readyState === WebSocket.OPEN) {
        life.networkOut++;
        ws.current.send(JSON.stringify(message));
      }
    },
    connected,
    detail?.room.id,
  );
  const building = useBuilding(
    (message) => {
      if (ws.current?.readyState === WebSocket.OPEN) {
        life.networkOut++;
        ws.current.send(JSON.stringify(message));
      }
    },
    connected,
    detail?.room.id,
  );
  const life = useLife((message) => {
    if(ws.current?.readyState === WebSocket.OPEN) {life.networkOut++;ws.current.send(JSON.stringify(message));}
  }, connected, detail?.room.id);
  life.selfId = user?.id ?? "";
  gathering.personalInventory = life.personal?.inventory ?? {};
  building.selfId = user?.id ?? "";
  building.players = players.filter((p) => p.world === "freetower");
  const serverOffset = useRef(0);
  useEffect(() => {
    if (detail?.state.freeTower) {
      gathering.applyState(detail.state.freeTower);
      building.applyState(detail.state.freeTower);
      life.applyState(detail.state.freeTower);
    }
  }, [detail?.state.freeTower, gathering]);
  useEffect(() => {
    const self = players.find((p) => p.id === user?.id);
    if (self) gathering.applyPlayer(self, serverOffset.current);
  }, [players, user?.id, gathering]);
  const activeIdentity = useRef(true);
  useEffect(() => { activeIdentity.current = true; return () => { activeIdentity.current = false; }; }, []);
  const requestError = (error: unknown) => {
    if (!activeIdentity.current) return;
    if (identityUser && recoverUnauthorized(error, () => onIdentityRevoked?.())) return;
    setError(error instanceof Error ? error.message : "请求失败，请重试");
  };
  const perform = async (task: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await task();
    } catch (e) {
      requestError(e);
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    let active = true;
    (identityUser ? Promise.resolve({ user: identityUser }) : api.me())
      .then(async ({ user }) => {
        if (!active) return;
        setUser(user);
        const r = await api.rooms();
        if (active) {
          setRooms(r.rooms);
          const remembered = sessionStorage.getItem(`freetower.room.${user.id}`);
          if (remembered && r.rooms.some((room) => room.id === remembered)) {
            const d = await api.room(remembered);
            if (active) setDetail(d);
          }
        }
      })
      .catch(error => { if (active) requestError(error); })
      .finally(() => {
        if (active) setChecking(false);
      });
    return () => {
      active = false;
    };
  }, []);
  const enter = (d: RoomDetail) => {
    sessionStorage.setItem(`freetower.room.${user?.id}`, d.room.id);
    motion.current = initialMotion();
    setDetail(d);
    setSettings(false);
    setChat([]);
    setError("");
  };
  useEffect(() => {
    if (!detail || !user) return;
    let disposed = false,
      timer: ReturnType<typeof setTimeout>,
      attempt = 0,
      spawned = false;
    const roomId = detail.room.id;
    const connect = () => {
      if (disposed) return;
      spawned = false;
      setConnected(false);
      const socket = new WebSocket(
        `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/rooms/${roomId}/ws?world=freetower`,
      );
      ws.current = socket;
      socket.onmessage = (e) => {
        if (disposed) return;
        let msg: ServerMessage;
        try {
          msg = JSON.parse(e.data);
        } catch {
          return;
        }
        life.networkIn++;
        if(msg.type === "life-self" || msg.type === "life-result" || msg.type === "life-error") {
          life.handle(msg);
          gathering.personalInventory = life.personal?.inventory ?? {};
        } else if (msg.type === "presence") {
          setPlayers(msg.players);
          const self = msg.players.find(
            (p) => p.id === user.id && p.world === "freetower",
          );
          if (self) {
            if (!spawned) {
              motion.current = {
                ...initialMotion(),
                position: { ...self.position },
                ...locomotionAt(self.position),
              };
              spawned = true;
            } else if (
              Math.hypot(
                self.position.x - motion.current.position.x,
                self.position.z - motion.current.position.z,
              ) > 2
            ) {
              motion.current.position = { ...self.position };
            }
            attempt = 0;
            setConnected(true);
          }
        } else if (
          msg.type === "building-result" ||
          msg.type === "building-error"
        ) {
          building.handle(msg);
        } else if (msg.type === "resource-hit") {
          gathering.hits[msg.resourceId] = msg;
          playGatherHit(msg);
          if (msg.depleted) {
            const node = gatheringConfig.nodes.find(
              (n) => n.id === msg.resourceId,
            );
            const d = node ? definitions[node.definitionId] : undefined;
            gathering.note(
              d
                ? d.drops
                    .map((drop) => `+${drop.quantity} ${drop.itemId}`)
                    .join(" · ")
                : "Resource gathered",
            );
          }
        } else if (msg.type === "gather-error") {
          gathering.action = null;
          gathering.note(msg.message);
        } else if (msg.type === "state") {
          serverOffset.current = msg.serverTime - Date.now();
          gathering.serverOffset = serverOffset.current;
          building.serverOffset = serverOffset.current;
          life.serverOffset = serverOffset.current;
          setDetail((d) =>
            d &&
            d.room.id === roomId &&
            (msg.state.version > d.state.version ||
              (msg.state.version === d.state.version &&
                msg.state.updatedAt >= d.state.updatedAt))
              ? { ...d, state: msg.state, serverTime: msg.serverTime }
              : d,
          );
        } else if (msg.type === "chat") setChat((c) => [...c.slice(-5), msg]);
        else if (msg.type === "roomDeleted") {
          sessionStorage.removeItem(`freetower.room.${user?.id}`);
          setDetail(null);
          setError("这个世界已被主人删除。");
          api
            .rooms()
            .then((r) => setRooms(r.rooms))
            .catch(error => { if (!disposed) requestError(error); });
        } else if (msg.type === "error") setError(msg.message);
      };
      socket.onclose = (event) => {
        if (disposed) return;
        if (event.code === 4001 && identityUser) { onIdentityRevoked?.(); return; }
        setConnected(false);
        setPlayers([]);
        const retry = async () => {
          if (identityUser) {
            const result = await reconnectSession(signal => identityAPI.me(signal), identityUser.id);
            if (disposed) return;
            if (result === "expired") { onIdentityRevoked?.(); return; }
          }
          if (!disposed) timer = setTimeout(connect, Math.min(5000, 500 + ++attempt * 700));
        };
        void retry();
      };
      socket.onerror = () => socket.close();
    };
    connect();
    return () => {
      disposed = true;
      clearTimeout(timer);
      ws.current?.close();
      ws.current = null;
      setConnected(false);
      setPlayers([]);
    };
  }, [detail?.room.id, user?.id]);
  const sendMove = useCallback((m: ReturnType<typeof movePayload>) => {
    if (ws.current?.readyState === WebSocket.OPEN)
      { life.networkOut++; ws.current.send(JSON.stringify(m)); }
  }, []);
  const submitAuth = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    void perform(async () => {
      const r = await api.auth(
        register ? "register" : "login",
        String(f.get("username")),
        String(f.get("password")),
      );
      setUser(r.user);
      setRooms((await api.rooms()).rooms);
    });
  };
  const sendChat = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const f = e.currentTarget,
      input = f.elements.namedItem("text") as HTMLInputElement;
    if (input.value.trim() && ws.current?.readyState === WebSocket.OPEN) {
      life.networkOut++;
      ws.current.send(
        JSON.stringify({ type: "chat", text: input.value.trim() }),
      );
      input.value = "";
    }
  };
  return (
    <main className="ft-app">
      <div className="ft-world">
        <FreeTowerWorld
          life={life}
          building={building}
          gathering={gathering}
          players={players}
          selfId={user?.id}
          motion={motion}
          ready={connected}
          onMove={sendMove}
          onReadout={setReadout}
          debug={debug}
        />
      </div>
      {detail ? (
        <>
          <LifeHUD controller={life} gathering={gathering} building={building} debug={debug}/>
          <GatheringHUD controller={gathering} debug={debug} />
          <BuildingHUD
            controller={building}
            gathering={gathering}
            debug={debug}
          />
          <header className="ft-heading">
            <div className="ft-wordmark">
              <span className="ft-sigil">◇</span> FREE<span>TOWER</span>
            </div>
            <p>{readout?.region ?? "Dawn Grassland · 晨光原野"}</p>
          </header>
          <div className="ft-topright">
            <span className="ft-online">
              <i className={connected ? "" : "offline"} />
              {connected
                ? `${players.filter((p) => p.world === "freetower").length} Online`
                : "重新连接中…"}
            </span>
            <button
              aria-label="Settings"
              onClick={() => setSettings(!settings)}
            >
              ⚙
            </button>
          </div>
          <div className="ft-compass">
            N <span>·</span> ANCIENT WORLD <span>·</span> S
          </div>
          {readout && readout.mode !== "land" && (
            <div className="ft-water-state">
              {readout?.mode === "swimming"
                ? "≈ Swimming · 游泳"
                : "≈ Wading · 涉水"}
            </div>
          )}
          <section className="ft-chat" aria-label="Multiplayer chat">
            <div className="ft-chat-log" aria-live="polite">
              {chat.map((c, i) => (
                <p key={`${c.at}-${i}`}>
                  <b>{publicCharacterName(c)}</b> {c.text}
                </p>
              ))}
            </div>
            <form onSubmit={sendChat}>
              <input
                name="text"
                aria-label="Chat message"
                placeholder="Enter a message… / 聊天"
                maxLength={180}
                disabled={!connected}
              />
              <button disabled={!connected}>↵</button>
            </form>
          </section>
          <footer className="ft-controls">
            <span>
              <kbd>W A S D</kbd> Move
            </span>
            <span>
              <kbd>Shift</kbd> Run
            </span>
            <span>
              <kbd>Space</kbd> Jump
            </span>
            <span>Click to move</span>
            <span>Right drag · Camera</span>
            <span>Scroll · Zoom</span>
          </footer>
          {settings && (
            <aside className="ft-settings">
              <h2>World settings</h2>
              <p>{detail.room.name}</p>
              <p>
                邀请码 <strong>{detail.room.code}</strong>
              </p>
              <p>14:00 · Bright daylight</p>
              <p>
                [ / ] · 分步旋转镜头
                <br />
                Home · 重置镜头
              </p>
              <p>Position resets at reconnect. World progress stays saved.</p>
              <button
                onClick={() => {
                  sessionStorage.removeItem(`freetower.room.${user?.id}`);
                  setDetail(null);
                  void perform(async () => setRooms((await api.rooms()).rooms));
                }}
              >
                返回世界列表
              </button>
              {!identityUser && <a href="/?legacyAuth=1&experience=little-flock">DEV · Little Flock 兼容入口</a>}
            </aside>
          )}
          <div className="ft-online-names" aria-label="Online characters">{players.filter(p => p.world === "freetower").map(p => <span key={p.id}>{publicCharacterName(p)}</span>)}</div>
          {debug && readout && (
            <output className="ft-debug">
              FPS {readout.fps} · Calls {readout.calls} · Triangles{" "}
              {readout.triangles.toLocaleString()}
              <br />X {readout.position.x.toFixed(2)} · Z{" "}
              {readout.position.z.toFixed(2)}
              <br />
              {readout.mode} · {readout.water}
              <br />
              Shared state v{detail.state.version}
            </output>
          )}
        </>
      ) : (
        <section className="ft-entry">
          <div className="ft-entry-eyebrow">A WORLD BEFORE CIVILIZATION</div>
          <h1>
            FREE<span>TOWER</span>
          </h1>
          <p className="ft-intro">循着海风，走向远古之塔。</p>
          {checking ? (
            <p>正在连接世界…</p>
          ) : !user ? (
            <form onSubmit={submitAuth}>
              <h2>{register ? "开始你的旅程" : "欢迎归来，探索者"}</h2>
              <label>
                用户名
                <input
                  name="username"
                  autoComplete="username"
                  required
                  minLength={3}
                  maxLength={24}
                />
              </label>
              <label>
                密码
                <input
                  name="password"
                  type="password"
                  autoComplete={register ? "new-password" : "current-password"}
                  required
                  minLength={6}
                />
              </label>
              <button className="ft-primary" disabled={busy}>
                {register ? "创建账号" : "登录"}
              </button>
              <button
                type="button"
                className="ft-link"
                onClick={() => setRegister(!register)}
              >
                {register ? "已有账号 · 登录" : "初次到访 · 注册"}
              </button>
            </form>
          ) : (
            <div className="ft-lobby">
              <h2>{publicCharacterName(user)}，选择一个世界</h2>
              <div className="ft-room-list">
                {rooms.map((r) => (
                  <button
                    key={r.id}
                    disabled={busy}
                    onClick={() =>
                      void perform(async () => enter(await api.room(r.id)))
                    }
                  >
                    <span>{r.name}</span>
                    <small>{r.memberCount} 位成员 · Enter →</small>
                  </button>
                ))}
              </div>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  const f = new FormData(e.currentTarget);
                  void perform(async () =>
                    enter(await api.create(String(f.get("name")))),
                  );
                }}
              >
                <input
                  name="name"
                  aria-label="World name"
                  placeholder="新世界名称"
                  required
                  maxLength={32}
                />
                <button disabled={busy}>创建世界</button>
              </form>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  const f = new FormData(e.currentTarget);
                  void perform(async () =>
                    enter(await api.join(String(f.get("code")))),
                  );
                }}
              >
                <input
                  name="code"
                  aria-label="Invite code"
                  placeholder="邀请码"
                  required
                />
                <button disabled={busy}>加入世界</button>
              </form>
              <button
                className="ft-link"
                onClick={() =>
                  void perform(async () => {
                    if (onIdentityDisconnect) { await onIdentityDisconnect(); return; }
                    await api.logout();
                    sessionStorage.removeItem(`freetower.room.${user?.id}`);
                    setUser(null);
                    setRooms([]);
                  })
                }
              >
                退出账号
              </button>
            </div>
          )}
        </section>
      )}
      {error && (
        <div className="ft-error" role="alert">
          {error}
          <button onClick={() => setError("")}>×</button>
        </div>
      )}
    </main>
  );
}
