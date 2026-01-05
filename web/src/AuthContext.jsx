import React, { createContext, useContext, useEffect, useState } from "react";
import { login as apiLogin, me as apiMe, registerAdmin } from "./api";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [token, setToken] = useState(localStorage.getItem("sampmail_token") || "");
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(!!token);

  // Multi-tenancy State
  const [organizations, setOrganizations] = useState([]);
  const [currentOrganization, setCurrentOrganization] = useState(null);

  useEffect(() => {
    if (!token) {
      setUser(null);
      setOrganizations([]);
      setCurrentOrganization(null);
      setLoading(false);
      return;
    }
    (async () => {
      try {
        const u = await apiMe(); // Note: apiMe might need update to return orgs if session persists? 
        // Actually, apiMe usually returns minimal user info. 
        // We might need to fetch orgs separately or update apiMe. 
        // For now, let's assume we load orgs from localStorage or fetch them.
        // Ideally, apiMe should return "organizations" too.
        setUser(u);

        // Load stored org ID
        const storedOrgId = localStorage.getItem("sampmail_org_id");
        if (storedOrgId && u.organizations) {
          const org = u.organizations.find(o => o.id === parseInt(storedOrgId));
          if (org) setCurrentOrganization(org);
        }
      } catch {
        localStorage.removeItem("sampmail_token");
        localStorage.removeItem("sampmail_org_id"); // Clear org
        setToken("");
        setUser(null);
      } finally {
        setLoading(false);
      }
    })();
  }, [token]);

  const handleLogin = async (email, password) => {
    const res = await apiLogin(email, password);

    // FIX: Check if 2FA is required before setting token
    if (res.requires_2fa) {
      return res; // Return early, let component handle 2FA step
    }

    // Normal login success
    finishLogin(res);
    return res;
  };

  const finishLogin = (data) => {
    localStorage.setItem("sampmail_token", data.token);
    setToken(data.token);

    const userData = {
      email: data.email,
      is_super_admin: data.is_super_admin,
      organizations: data.organizations
    };
    setUser(userData);
    setOrganizations(data.organizations || []);

    // Auto-select Organization
    // 1. If SuperAdmin and no orgs ? Global View (currentOrganization = null)
    // 2. If User has orgs, select first or stored
    if (data.organizations && data.organizations.length > 0) {
      const storedOrgId = localStorage.getItem("sampmail_org_id");
      let org = null;

      if (storedOrgId) {
        org = data.organizations.find(o => o.id === parseInt(storedOrgId));
      }

      if (!org) {
        org = data.organizations[0]; // Default to first
      }

      selectOrganization(org);
    } else {
      // No orgs. If superadmin, that's fine (Global). If not, user has no access.
      localStorage.removeItem("sampmail_org_id");
      setCurrentOrganization(null);
    }
  };

  // FIX: New helper for 2FA verification step
  const verify2FA = async (tempToken, code) => {
    const res = await fetch('/api/auth/verify-2fa', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Temp-Token': tempToken
      },
      body: JSON.stringify({ code })
    });

    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || 'Verification failed');
    }

    finishLogin(data);
    return data;
  };

  const handleRegister = async (email, password) => {
    const res = await registerAdmin(email, password);
    // Register likely doesn't verify Org yet. 
    finishLogin(res);
  };

  const logout = () => {
    localStorage.removeItem("sampmail_token");
    localStorage.removeItem("sampmail_org_id");
    setToken("");
    setUser(null);
    setOrganizations([]);
    setCurrentOrganization(null);
  };

  const selectOrganization = (org) => {
    if (!org) {
      localStorage.removeItem("sampmail_org_id");
      setCurrentOrganization(null);
      return;
    }
    localStorage.setItem("sampmail_org_id", org.id);
    setCurrentOrganization(org);
    // Optional: Force reload to ensure all data is refreshed?
    // window.location.reload(); 
    // Better: Just let React state update trigger re-renders of components using Context.
  };

  return (
    <AuthContext.Provider
      value={{
        token,
        user,
        loading,
        organizations,
        currentOrganization,
        selectOrganization,
        login: handleLogin,
        verify2FA,
        register: handleRegister,
        logout
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
