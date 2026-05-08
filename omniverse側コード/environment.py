# Comments are in English only.

from pxr import UsdGeom
import omni.usd

ENV_USD = "file:///C:/Users/icnl2/Downloads/classroom/classroom_realsize.usd"
ENV_PRIM_PATH = "/World/Environment"

stage = omni.usd.get_context().get_stage()
if stage is None:
    raise RuntimeError("No USD stage is open. Create/open a stage first.")

# Create an Xform prim and reference the environment USD
env_prim = stage.GetPrimAtPath(ENV_PRIM_PATH)
if not (env_prim and env_prim.IsValid()):
    env_prim = UsdGeom.Xform.Define(stage, ENV_PRIM_PATH).GetPrim()

refs = env_prim.GetReferences()
refs.ClearReferences()
refs.AddReference(ENV_USD)

print("Environment loaded:", ENV_USD)
