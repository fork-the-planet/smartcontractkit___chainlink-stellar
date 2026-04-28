//! Role-based access control for RBACTimelock.
//!
//! Roles are stored in persistent storage (unlike `common-authorization::AccessControl`
//! which uses instance storage) so they survive ledger archival.
//!
//! Security model mirrors Solidity `AccessControl`:
//! - `ADMIN_ROLE` is self-administered: only admins may grant/revoke any role.
//! - `require_role_or_admin` mirrors Solidity `onlyRoleOrAdminRole`.

use soroban_sdk::{Address, Env, Map, Symbol, Vec};

use crate::error::TimelockError;
use crate::events::{RoleGrantedEvent, RoleRevokedEvent};
use crate::storage::{get_roles_map, set_roles_map};
use crate::types::ADMIN_ROLE;

/// Returns true if `account` is a member of `role`.
pub fn has_role(env: &Env, role: Symbol, account: &Address) -> bool {
    let roles = get_roles_map(env);
    let members: Vec<Address> = roles.get(role).unwrap_or(Vec::new(env));
    for m in members.iter() {
        if m == *account {
            return true;
        }
    }
    false
}

/// Require `caller` to hold `role` OR `ADMIN_ROLE`, then call `caller.require_auth()`.
///
/// Mirrors Solidity `onlyRoleOrAdminRole`.
pub fn require_role_or_admin(
    env: &Env,
    caller: &Address,
    role: Symbol,
) -> Result<(), TimelockError> {
    if has_role(env, ADMIN_ROLE, caller) || has_role(env, role, caller) {
        caller.require_auth();
        return Ok(());
    }
    Err(TimelockError::NotAuthorized)
}

/// Require `caller` to hold `ADMIN_ROLE`, then call `caller.require_auth()`.
pub fn require_admin(env: &Env, caller: &Address) -> Result<(), TimelockError> {
    if !has_role(env, ADMIN_ROLE, caller) {
        return Err(TimelockError::NotAuthorized);
    }
    caller.require_auth();
    Ok(())
}

/// Add `account` to `role`. Emits `RoleGrantedEvent`.
///
/// Callers must have already validated authorization before calling this.
pub fn grant_role_internal(env: &Env, role: Symbol, account: &Address, sender: &Address) {
    let mut roles: Map<Symbol, Vec<Address>> = get_roles_map(env);
    let mut members: Vec<Address> = roles.get(role.clone()).unwrap_or(Vec::new(env));
    // idempotent: only add if not already present
    let mut found = false;
    for m in members.iter() {
        if m == *account {
            found = true;
            break;
        }
    }
    if !found {
        members.push_back(account.clone());
        roles.set(role.clone(), members);
        set_roles_map(env, &roles);
        RoleGrantedEvent {
            role,
            account: account.clone(),
            sender: sender.clone(),
        }
        .publish(env);
    }
}

/// Remove `account` from `role`. Emits `RoleRevokedEvent` if the account had the role.
///
/// Callers must have already validated authorization before calling this.
pub fn revoke_role_internal(env: &Env, role: Symbol, account: &Address, sender: &Address) {
    let mut roles: Map<Symbol, Vec<Address>> = get_roles_map(env);
    let members: Vec<Address> = match roles.get(role.clone()) {
        Some(m) => m,
        None => return,
    };
    let mut new_members: Vec<Address> = Vec::new(env);
    let mut found = false;
    for m in members.iter() {
        if m == *account {
            found = true;
        } else {
            new_members.push_back(m);
        }
    }
    if found {
        roles.set(role.clone(), new_members);
        set_roles_map(env, &roles);
        RoleRevokedEvent {
            role,
            account: account.clone(),
            sender: sender.clone(),
        }
        .publish(env);
    }
}

/// Get all members of `role`.
pub fn get_role_members(env: &Env, role: Symbol) -> Vec<Address> {
    let roles = get_roles_map(env);
    roles.get(role).unwrap_or(Vec::new(env))
}
