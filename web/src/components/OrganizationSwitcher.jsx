import React, { useState, useRef, useEffect } from "react";
import { useAuth } from "../AuthContext";
import { ChevronDown, Check, Building, Plus, Settings } from "lucide-react";

export default function OrganizationSwitcher() {
    const { user, organizations, currentOrganization, selectOrganization } = useAuth();
    const [isOpen, setIsOpen] = useState(false);
    const dropdownRef = useRef(null);

    // Close dropdown when clicking outside
    useEffect(() => {
        function handleClickOutside(event) {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
                setIsOpen(false);
            }
        }
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, []);

    if (!user) return null;

    return (
        <div className="relative" ref={dropdownRef}>
            <button
                onClick={() => setIsOpen(!isOpen)}
                className="flex items-center space-x-2 px-3 py-2 rounded-lg hover:bg-gray-800 transition-colors w-full"
            >
                <div className="w-8 h-8 rounded bg-blue-600 flex items-center justify-center text-white font-bold shrink-0">
                    {currentOrganization ? currentOrganization.name.charAt(0).toUpperCase() : <Building size={16} />}
                </div>
                <div className="flex-1 text-left overflow-hidden">
                    <div className="text-sm font-medium truncate text-white">
                        {currentOrganization ? currentOrganization.name : "Select Org"}
                    </div>
                    <div className="text-xs text-gray-400 truncate">
                        {currentOrganization ? currentOrganization.role : (user.is_super_admin ? "Global Admin" : "No Access")}
                    </div>
                </div>
                <ChevronDown size={16} className={`text-gray-400 transition-transform ${isOpen ? "rotate-180" : ""}`} />
            </button>

            {isOpen && (
                <div className="absolute top-full left-0 mt-2 w-64 bg-gray-900 border border-gray-700 rounded-lg shadow-xl z-50 overflow-hidden">
                    <div className="p-2 border-b border-gray-700">
                        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider px-2">Organizations</span>
                    </div>

                    <div className="max-h-60 overflow-y-auto py-1">
                        {organizations.length === 0 && (
                            <div className="px-4 py-3 text-sm text-gray-400 text-center">
                                No organizations found.
                            </div>
                        )}

                        {organizations.map((org) => (
                            <button
                                key={org.id}
                                onClick={() => {
                                    selectOrganization(org);
                                    setIsOpen(false);
                                    // Optional: Redirect to dashboard home
                                    // window.location.href = "/";
                                }}
                                className={`w-full flex items-center justify-between px-4 py-2 text-sm hover:bg-gray-800 transition-colors ${currentOrganization && currentOrganization.id === org.id ? "text-blue-400 bg-gray-800/50" : "text-gray-300"
                                    }`}
                            >
                                <div className="flex items-center space-x-3 truncate">
                                    <div className="w-6 h-6 rounded bg-gray-700 flex items-center justify-center text-xs text-white">
                                        {org.name.charAt(0)}
                                    </div>
                                    <span>{org.name}</span>
                                </div>
                                {currentOrganization && currentOrganization.id === org.id && <Check size={16} />}
                            </button>
                        ))}
                    </div>

                    {user.is_super_admin && (
                        <div className="border-t border-gray-700 p-2 bg-gray-800/50">
                            <button
                                onClick={() => {
                                    // TODO: Implement Create Org Modal
                                    setIsOpen(false);
                                    alert("Create Organization UI coming soon");
                                }}
                                className="flex items-center space-x-2 w-full px-2 py-1.5 text-sm text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
                            >
                                <Plus size={16} />
                                <span>Create Organization</span>
                            </button>
                            <button
                                onClick={() => {
                                    setIsOpen(false);
                                    // Navigate to Superadmin Dashboard
                                }}
                                className="flex items-center space-x-2 w-full px-2 py-1.5 text-sm text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors mt-1"
                            >
                                <Settings size={16} />
                                <span>Manage Tenants</span>
                            </button>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
