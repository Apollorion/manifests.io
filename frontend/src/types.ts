export interface Product {
  name: string;
  versions: string[];
  defaultVersion?: string;
}

export interface Link {
  label: string;
  href: string;
  circular?: boolean;
}

export interface Row {
  name: string;
  type: string;
  description: string;
  required?: boolean;
  href?: string;
  circular?: boolean;
  constraints?: string[];
  variants?: Link[];
}

export interface Page {
  item: string;
  version: string;
  resource?: string;
  title: string;
  description: string;
  path?: string;
  pointer?: string;
  trail?: string;
  resources: Row[];
  otherVersions: Link[];
  breadcrumbs: Link[];
  variants: Link[];
  catalog: Product[];
  canonical: string;
  error?: string;
}
