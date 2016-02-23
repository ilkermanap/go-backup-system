from flask import Flask,session, request, flash, url_for, redirect, render_template, abort ,g
from flask.ext.login import login_user , logout_user , current_user , login_required
from datetime import datetime
from flask_sqlalchemy import SQLAlchemy
from flask.ext.login import LoginManager
from werkzeug import secure_filename
import os
import hashlib


from OpenSSL import SSL
context = SSL.Context(SSL.TLSv1_METHOD)
context.use_privatekey_file('server.key')
context.use_certificate_file('server.crt') 


BACKUP = "/home/ilker/src/yedekleme-sunucu/wsgi/static/backup"
app = Flask(__name__)
app.config.from_pyfile('backupserver.cfg')
db = SQLAlchemy(app)


login_manager = LoginManager()
login_manager.init_app(app)

login_manager.login_view = 'giris'



class Musteri(db.Model):
    __tablename__ = 'musteri'
    id = db.Column('id', db.Integer, db.Sequence('musteri_id_seq', start=1, increment=1),primary_key=True)
    adi = db.Column('adi', db.String(60), index=True)
    email = db.Column(db.String(60), unique=True, index=True)
    passwd = db.Column(db.String(64))
    kayit_tarihi = db.Column('kayit_tarihi' , db.DateTime)

    def __init__(self, adi, sifre, email):
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
 
    def __repr__(self):
        return '<Musteri %r>' % (self.adi)

@app.route('/kayit' , methods=['GET','POST'])
def kayit():
    if request.method == 'GET':
        return render_template('kayit.html')
    musteri = Musteri(request.form['username'] , request.form['password'],request.form['email'])
    db.session.add(musteri)
    db.session.commit()
    os.mkdir("%s/%s" % (BACKUP, hashlib.sha256(request.form['email']).hexdigest()))
    flash('Kullanici basariyla eklendi')
    return redirect(url_for('giris'))
 

@app.route('/dosya', methods=['POST'])
def dosya():
    if request.method == "POST":
        email = request.form['email']
        password = request.form['sifre']
        print email, password, request.files['file'].filename
        kullanici = Musteri.query.filter_by(email=email,passwd=password).first()
        print kullanici
        if kullanici is  None:
            return "hatali kullanici"

        tarih = request.form['tarih']
        file = request.files['file']
        fname = secure_filename(file.filename)
        isim = fname.split("/")[-1]
        dizin = "%s/%s/%s" % (BACKUP, hashlib.sha256(email).hexdigest(), tarih)
        os.system("mkdir -p %s" % dizin)
        f = os.path.join(dizin, isim)
        file.save(f)
        return os.popen("sha256sum %s" % f, "r").readlines()[0].split()[0].strip()

@app.route('/giris',methods=['GET','POST'])
def giris():
    if request.method == 'GET':
        return render_template('giris.html')
    email = request.form['username']
    password = request.form['password']
    kayitli = Musteri.query.filter_by(email=email,passwd=password).first()
    if kayitli is None:
        flash('Email ya da sifre yanlis' , 'error')
        return redirect(url_for('giris'))
    login_user(kayitli)
    flash('Logged in successfully')
    return redirect(request.args.get('next') or url_for('index'))


def kullanici_onay(rq):
    musteri_email = rq.args.get('email')
    musteri_sifre = rq.args.get('sifre')
    return(Musteri.query.filter_by(email=email,passwd=musteri_sifre).first())


@app.route('/gonder', methods = ['POST'])
def gonder():
    kayitli = kullanici_onay(request)
    if kayitli is None:
        flash('Email ya da sifre yanlis' , 'error')
    else:
        return("basarili")

@app.route('/katalog', methods = ['POST'])
def katalog():
    kayitli = kullanici_onay(request)
    if kayitli is None:
        flash('Email ya da sifre yanlis' , 'error')
    else:
        return("basarili")

@login_manager.user_loader
def load_user(id):
    return Musteri.query.get(int(id))


@app.route('/cikis')
def cikis():
    logout_user()
    return redirect(url_for('index')) 

@app.route('/')
@login_required
def index():
    return render_template('index.html', musteri=Musteri.query.all())


if __name__ == '__main__':
    #db.create_all()
    app.run(host="0.0.0.0", ssl_context=context, debug=True)
    
