import { useSyncExternalStore } from 'react';
import type { IAccount } from '@/data/mock';

const API_BASE = (import.meta.env.VITE_GP_API_BASE ?? '').replace(/\/$/, '');
const ROLE_LABEL: Record<string,string> = { owner: '团队所有者', admin: '管理员', tester: '测试工程师', viewer: '访客' };
type ApiUser = { id:number; username:string; display_name:string; email:string; team_id:number; role:string };
function account(u:ApiUser):IAccount { return { id:String(u.id), name:u.display_name, email:u.email, role:ROLE_LABEL[u.role]??u.role, username:u.username, phone:'', wechat:'', avatarColor:'from-emerald-500 to-teal-600', via:'password' }; }

let current:IAccount|null=null;
let ready=false;
// Monotonic auth epoch: a slow /auth/me response must never overwrite a newer
// explicit login/logout. This removes the restore-vs-login race on refresh.
let authEpoch=0;
const listeners=new Set<()=>void>();
const emit=()=>listeners.forEach((l)=>l());

export const loginStore={
  get:()=>current,
  isReady:()=>ready,
  subscribe:(l:()=>void)=>{listeners.add(l);return()=>{listeners.delete(l);};},
  async restore(){
    const epoch=authEpoch;
    let restored:IAccount|null|undefined;
    try {
      const r=await fetch(`${API_BASE}/auth/me`,{credentials:'same-origin'});
      if(r.ok) restored=account(((await r.json()) as {user:ApiUser}).user);
      else if(r.status===401) restored=null;
    } catch { /* keep the current view on transient network errors */ }
    if(epoch!==authEpoch) return;
    if(restored!==undefined) current=restored;
    ready=true;emit();
  },
  async login(username:string,password:string){
    authEpoch++;
    const r=await fetch(`${API_BASE}/auth/login`,{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({username,password})});
    if(!r.ok) throw new Error('账号或密码错误');
    const body=await r.json() as {user:ApiUser}; current=account(body.user);ready=true;emit();return current;
  },
  async logout(){ authEpoch++; await fetch(`${API_BASE}/auth/logout`,{method:'POST',credentials:'same-origin'});current=null;ready=true;emit(); },
};
void loginStore.restore();
export function useCurrentUser(){return useSyncExternalStore(loginStore.subscribe,loginStore.get);}
export function useLoginReady(){return useSyncExternalStore(loginStore.subscribe,loginStore.isReady);}
