import { Link, type LinkProps, NavLink, type NavLinkProps, useLocation } from "react-router-dom";
import { preserveEnvironment } from "@/features/environment-context/environment";

export function EnvironmentLink({ to, ...props }: LinkProps) {
	const { search } = useLocation();
	return <Link {...props} to={preserveEnvironment(to, search)} />;
}

export function EnvironmentNavLink({ to, ...props }: NavLinkProps) {
	const { search } = useLocation();
	return <NavLink {...props} to={preserveEnvironment(to, search)} />;
}
