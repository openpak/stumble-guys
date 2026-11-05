#!/usr/bin/env python3
"""Fill the cosmetics, level and bot parts of cmd/stumbled/shared.json from the game's own data.

The real service's game config is not available, so the ids the client looks up are read from what the game
ships: the ScriptableObjects in Data/data.unity3d (SkinDef, AnimationDef, FootstepsDef, EmoteDefinitionAsset,
LevelDef). One run writes one consistent set, so a game update is a re-run:

    python3 gen_catalog.py <extracted RomFS dir> [--shared cmd/stumbled/shared.json] [--no-rounds] [--no-bots]

<extracted RomFS dir> holds Data/data.unity3d. Needs UnityPy (pip install UnityPy). Output is deterministic.
"""
import argparse, json, re, struct, sys

import UnityPy


def class_of(scripts, raw):
    return scripts.get(struct.unpack_from('<q', raw, 20)[0]) if len(raw) >= 28 else None


def strings(raw):
    """The length-prefixed strings of a serialized ScriptableObject, in field order."""
    out, i = [], 28
    while i + 4 <= len(raw):
        n = struct.unpack_from('<I', raw, i)[0]
        if 0 < n < 200 and i + 4 + n <= len(raw) and all(32 <= b < 127 for b in raw[i + 4:i + 4 + n]):
            out.append(raw[i + 4:i + 4 + n].decode())
            i += 4 + (n + 3) // 4 * 4
        else:
            i += 4
    return out


def cosmetic(id_, version="0.1"):
    return {"Hidden": False, "NoGacha": True, "FriendlyName": id_, "Rarity": "COMMON", "Tier": 1, "Version": version,
            "Platforms": ["Switch"], "MinVersion": "0.0.0", "MaxVersion": "999.0.0"}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('romfs')
    ap.add_argument('--shared', default='cmd/stumbled/shared.json')
    ap.add_argument('--no-rounds', action='store_true')
    ap.add_argument('--no-bots', action='store_true')
    a = ap.parse_args()
    env = UnityPy.load(f'{a.romfs}/Data/data.unity3d')
    scripts = {o.path_id: o.read().m_ClassName for o in env.objects if o.type.name == 'MonoScript'}
    skins, steps, anims, emotes, levels = [], [], [], [], []
    for o in env.objects:
        if o.type.name != 'MonoBehaviour':
            continue
        raw = o.get_raw_data()
        cn = class_of(scripts, raw)
        if cn not in ('SkinDef', 'AnimationDef', 'FootstepsDef', 'EmoteDefinitionAsset', 'LevelDef'):
            continue
        s = strings(raw)
        if cn == 'SkinDef':
            f = [x for x in s if re.fullmatch(r'SKIN\d+', x)]
            if f:
                skins.append(f[0])
        elif cn == 'AnimationDef' and len(s) > 1 and re.fullmatch(r'animation\d+', s[1]):
            anims.append(s[1])
        elif cn == 'FootstepsDef':
            f = [x for x in s if x.startswith('footsteps_')]
            if f:
                steps.append(f[0])
        elif cn == 'EmoteDefinitionAsset':
            f = [x for x in s if re.fullmatch(r'emote_[a-z0-9_]+', x)]
            if f:
                emotes.append(f[0])
        elif cn == 'LevelDef' and len(s) > 3:
            levels.append({"id": s[1], "friendly": s[2], "scene": s[3], "tags": s[5:7]})
    key = lambda v: (len(v), v)
    skins, steps, anims, emotes = (sorted(set(x), key=key) for x in (skins, steps, anims, emotes))
    cfg = json.load(open(a.shared))
    cfg['Skins_v4'] = [dict(SkinID=x, IsFallback=(x == 'SKIN1'), **cosmetic(x)) for x in skins]
    cfg['Footsteps_v2'] = [dict(ID=x, **cosmetic(x)) for x in steps]
    cfg['Animations_v2'] = [dict(ID=x, **cosmetic(x)) for x in anims]
    cfg['Emotes_v2'] = [dict(ID=x, InternalID=i + 1, **cosmetic(x)) for i, x in enumerate(emotes)]
    if not a.no_bots:
        def pct(ids): return [{"CosmeticIds": ids, "PercentageOfBotsUsingThem": 100}]
        cfg['LevelsSettings'] = {"BotVisuals": [{
            "StartDateTime": "2020-01-01T00:00:00Z", "EndDateTime": "2099-01-01T00:00:00Z", "ApplyInEveryGameMode": True,
            "SpecificEventId": "", "InitialMinTimerToShowEmote": 5, "InitialMaxTimerToShowEmote": 15,
            "MinTimerToShowEmote": 10, "MaxTimerToShowEmote": 30, "PercentageToShowEmoteAfterBeenStumbled": 20,
            "PercentageToShowEmoteAfterBeenHitWithAbility": 20, "PercentageToShowEmoteRandomly": 10,
            "Skins": pct(['SKIN1']), "Footsteps": pct(['footsteps_smoke']), "SocialEmotes": pct(emotes[:4]),
            "VictoryAnimations": pct(['animation1'])}]}
    if not a.no_rounds:
        race = [l for l in levels if 'RACE_TO_FINISH' in l['tags'] and l['id'] in ('IPL_047_Flame', 'SSL_015_slides')]
        cfg['RoundLevels_v2'] = [{"LevelID": l['id'], "AllowedRounds": [1, 2, 3, 4, 5], "MatchmakingPool": p}
                                 for l in sorted(race, key=lambda l: l['id']) for p in (1, 2, 3)]
    with open(a.shared, 'w') as f:
        json.dump(cfg, f, indent=1)
        f.write('\n')
    print(f'skins {len(skins)} footsteps {len(steps)} animations {len(anims)} emotes {len(emotes)} levelDefs {len(levels)}',
          file=sys.stderr)


if __name__ == '__main__':
    main()
