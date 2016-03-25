from flask import Flask,session, request, flash, url_for, redirect, render_template, abort ,g, jsonify
from flask.ext.login import login_user , logout_user , current_user , login_required
from datetime import datetime
from flask_sqlalchemy import SQLAlchemy
from sqlalchemy.orm import relationship
from sqlalchemy.ext.hybrid import hybrid_property
from flask.ext.login import LoginManager
from flask.ext.bcrypt import Bcrypt
from werkzeug import secure_filename
import os
import hashlib
import glob
import json

from OpenSSL import SSL
context = SSL.Context(SSL.TLSv1_METHOD)
context.use_privatekey_file('verimiz.key')
context.use_certificate_file('verimiz.crt') 


BACKUP = "/storage"
app = Flask(__name__)
app.config.from_pyfile('backupserver.cfg')
db = SQLAlchemy(app)
bcrypt = Bcrypt(app)


login_manager = LoginManager()
login_manager.init_app(app)

login_manager.login_view = 'giris'

def kullanici_onay(em,sf):
    u = Musteri.query.filter_by(email=em).first()
    if u  is not None:
        if u.is_correct_passwd(sf) == True:
            return u
    return None

class Katalog:
    def __init__(self, email):
        self.dizin = "%s/%s" % (BACKUP,hashlib.sha256(email).hexdigest())
        self.dosyalar = {}
        self.dizin_kontrol()
    
    def dizin_kontrol(self):
        for tarih  in glob.glob("%s/*" % self.dizin):
            trh = tarih.split("/")[-1]
            t = {}
            for katalog in glob.glob("%s/*.katalog.*" % tarih):
                print "----\n",katalog, "\n------"
                parts = glob.glob(katalog.replace(".katalog.bz2.enc","-*.tar"))
                boy = 0
                for p in  parts:
                    boy += os.stat(p).st_size / (1000 * 1000 * 1.0)                

                t[katalog.split("/")[-1]] = boy
            self.dosyalar[trh] = t


class Musteri(db.Model):
    __tablename__ = 'musteri'
    id = db.Column('id', db.Integer,primary_key=True)
    adi = db.Column('adi', db.String(60), index=True)
    email = db.Column(db.String(60), unique=True, index=True)
    _passwd = db.Column(db.String(64))
    kayit_tarihi = db.Column('kayit_tarihi' , db.DateTime)
    cihazlar = relationship

    @hybrid_property
    def passwd(self):
        return self._passwd

    @passwd.setter
    def _set_passwd(self, plaintext):
        self._passwd = bcrypt.generate_password_hash(plaintext)

    def __init__(self, adi, email, sifre):
        self.adi = adi
        self.email = email
        self.passwd = sifre
        self.kayit_tarihi = datetime.utcnow()

    def is_authenticated(self):
        return True
 
    def is_active(self):
        return True
 
    def is_anonymous(self):
        return False
 
    def get_id(self):
        return unicode(self.id)

    def is_correct_passwd(self, plaintext):
        return bcrypt.check_password_hash(self._passwd, plaintext)
    
    def __repr__(self):
        return '<Musteri %r>' % (self.adi)

class Cihaz(db.Model):
    __tablename__ = "cihaz"
    id = db.Column("id", db.Integer, primary_key=True)
    no = db.Column("no", db.Integer)
    adi = db.Column("cihaz", db.String(100))
    musteri_id = db.Column(db.Integer, db.ForeignKey('musteri.id'))


    
@app.route('/kayit' , methods=['GET','POST'])
def kayit():
    if request.method == 'GET':
        return render_template('kayit.html')
    musteri = Musteri(request.form['username'],request.form['email'], request.form['password'])
    db.session.add(musteri)
    db.session.commit()
    dizin = "%s/%s" % (BACKUP, hashlib.sha256(request.form['email']).hexdigest())
    if not os.path.isdir(dizin):
        os.mkdir(dizin)
    flash('Kullanici basariyla eklendi')
    return redirect(url_for('giris'))
 

@app.route('/dosya', methods=['POST'])
def dosya():
    if request.method == "POST":
        email = request.form['email']
        password = request.form['sifre']
        kullanici = kullanici_onay(email,password)
        if kullanici is  None:
            return "H-004 Kullanici adi ya da sifresi hatali"
        
        tarih = request.form['tarih']
        file = request.files['file']
        fname = secure_filename(file.filename)
        isim = fname.split("/")[-1]
        dizin = "%s/%s/%s" % (BACKUP, hashlib.sha256(email).hexdigest(), tarih)
        os.system("mkdir -p %s" % dizin)
        f = os.path.join(dizin, isim)
        file.save(f)
        return os.popen("sha256sum %s" % f, "r").readlines()[0].split()[0].strip()


@app.route("/kontrol", methods = ['POST'])
def kontrol():
    if request.method == "GET":
        return "H-001 Yanlis metod"
    elif request.method == "POST":
        email = request.form['username']
        password = request.form['password']
        k = Musteri.query.filter_by(email=email).first()
        s = k.is_correct_passwd(password)
                
        if k is None:
            return "H-002 Kullanici tanimsiz"
        elif s is False:
            return "H-003 Yanlis sifre"
        else:
            return "T-001 Tamam"

@app.route('/giris',methods=['GET','POST'])
def giris():
    if request.method == 'GET':
        return render_template('giris.html')
    email = request.form['username']
    password = request.form['password']
    kayitli = kullanici_onay(email,password)
    if kayitli is None:
        flash('Email ya da sifre yanlis' , 'error')
        return redirect(url_for('giris'))
    login_user(kayitli)
    
    flash('Logged in successfully')
    return redirect(request.args.get('next') or url_for('index'))


@app.route('/gonder', methods = ['POST'])
def gonder():
    em = request.form['email']
    sf = request.form['sifre']
    kayitli = kullanici_onay(em,sf)
    if kayitli is None:
        return("H-004 Kullanici adi ya da sifresi hatali")
    else:
        dizin = "%s/%s/cihazlar.txt.enc"
        return dizin
        
@app.route("/cihaz_ekle", methods=['POST'])
def cihaz_ekle():
    em = request.form['email']
    sf = request.form['sifre']
    
    kayitli = kullanici_onay(em,sf)
    print "---",kayitli,"---"
    
    if kayitli is None:
        print "hatali"
        return "H-004  Kullanici adi ya da sifresi hatali"
    else:
        print 1
        cihaz_adi = request.form["cihaz_adi"]
        print(2, cihaz_adi)
        try:
            print 3
            cihaz_kontrol = Cihaz.query.filter_by(musteri_id = kayitli.id, adi = cihaz_adi).one()
        except:
            print 4
            cihaz_kontrol = None
        
        sayisi = Cihaz.query.filter_by(musteri_id = kayitli.id).count()
        print sayisi
        if cihaz_kontrol is None:
            c = Cihaz(no=sayisi+1, adi=cihaz_adi, musteri_id=kayitli.id)
            db.session.add(c)
            db.session.commit()
            return "%d numara ile cihaz %s eklendi" % (sayisi +1 , cihaz_adi)  
            
    
@app.route("/cihaz_listesi", methods=['POST'])
def cihaz_listesi():
    em = request.form['email']
    sf = request.form['sifre']
    
    kayitli = kullanici_onay(em,sf)
    
    if kayitli is None:
        return "H-004 Kullanici adi ya da sifresi hatali"
    else:
        cihazlar = Cihaz.query.filter_by(musteri_id = kayitli.id).all()
        t = {}
        for c in cihazlar:
            t[c.id] = {"adi":c.adi , "numara":c.no}
        return json.dumps(t)
                
    
@app.route('/katalog', methods = ['POST'])
def katalog():
    em = request.form['email']
    sf = request.form['sifre']
    kayitli = kullanici_onay(em,sf)
    if kayitli is None:
        flash('Email ya da sifre yanlis' , 'error')
    else:
        return("basarili")

@login_manager.user_loader
def load_user(id):
    return Musteri.query.get(int(id))

@app.before_request
def before_request():
    g.user = current_user

@app.route('/cikis')
def cikis():
    logout_user()
    return redirect(url_for('index')) 

@app.route('/')
@login_required
def index():
    return render_template('index.html', musteri=Musteri.query.all(), katalog = Katalog(g.user.email))


if __name__ == '__main__':
    #db.create_all()
    app.run(host="0.0.0.0", ssl_context=context, debug=True)
    
