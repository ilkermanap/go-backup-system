export namespace backup {
	
	export class BackupEntry {
	    id: number;
	    filename: string;
	    size: number;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.filename = source["filename"];
	        this.size = source["size"];
	        this.created_at = source["created_at"];
	    }
	}
	export class BackupStatus {
	    is_running: boolean;
	    last_backup: string;
	    next_backup: string;
	    files_count: number;
	    total_size: number;
	    device_id: number;
	    device_name: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_running = source["is_running"];
	        this.last_backup = source["last_backup"];
	        this.next_backup = source["next_backup"];
	        this.files_count = source["files_count"];
	        this.total_size = source["total_size"];
	        this.device_id = source["device_id"];
	        this.device_name = source["device_name"];
	    }
	}
	export class Device {
	    id: number;
	    name: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.created_at = source["created_at"];
	    }
	}
	export class QuotaInfo {
	    total_gb: number;
	    used_bytes: number;
	    used_gb: number;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new QuotaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_gb = source["total_gb"];
	        this.used_bytes = source["used_bytes"];
	        this.used_gb = source["used_gb"];
	        this.percent = source["percent"];
	    }
	}
	export class UsageInfo {
	    device_count: number;
	    backup_count: number;
	    total_size: number;
	    last_backup: string;
	
	    static createFrom(source: any = {}) {
	        return new UsageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_count = source["device_count"];
	        this.backup_count = source["backup_count"];
	        this.total_size = source["total_size"];
	        this.last_backup = source["last_backup"];
	    }
	}

}

export namespace config {
	
	export class Config {
	    server_url: string;
	    email: string;
	    password: string;
	    token: string;
	    device_id: number;
	    device_name: string;
	    encryption_key: string;
	    backup_dirs: string[];
	    blacklist: string[];
	    schedule_enabled: boolean;
	    start_time: string;
	    end_time: string;
	    interval_minutes: number;
	    skip_weekends: boolean;
	    chunk_size: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server_url = source["server_url"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.token = source["token"];
	        this.device_id = source["device_id"];
	        this.device_name = source["device_name"];
	        this.encryption_key = source["encryption_key"];
	        this.backup_dirs = source["backup_dirs"];
	        this.blacklist = source["blacklist"];
	        this.schedule_enabled = source["schedule_enabled"];
	        this.start_time = source["start_time"];
	        this.end_time = source["end_time"];
	        this.interval_minutes = source["interval_minutes"];
	        this.skip_weekends = source["skip_weekends"];
	        this.chunk_size = source["chunk_size"];
	    }
	}

}

export namespace main {
	
	export class CatalogFileInfo {
	    orig_path: string;
	    directory: string;
	    file_name: string;
	    latest_version: string;
	    version_count: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new CatalogFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orig_path = source["orig_path"];
	        this.directory = source["directory"];
	        this.file_name = source["file_name"];
	        this.latest_version = source["latest_version"];
	        this.version_count = source["version_count"];
	        this.size = source["size"];
	    }
	}
	export class FileVersionInfo {
	    timestamp: string;
	    size: number;
	    hash: string;
	
	    static createFrom(source: any = {}) {
	        return new FileVersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.size = source["size"];
	        this.hash = source["hash"];
	    }
	}
	export class LoginResult {
	    success: boolean;
	    user: any;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.user = source["user"];
	        this.token = source["token"];
	    }
	}

}

