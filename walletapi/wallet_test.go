// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package walletapi

import "fmt"
import "testing"
import "strings"

// we are covering atleast one test case each for all supported languages

func Test_Wallet_Generation_and_Recovery(t *testing.T) {

	tests := []struct {
		name       string
		seed       string
		secret_key string
		public_key string
		Address    string
	}{
		{
			name:       "English",
			seed:       "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons randomly",
			secret_key: "b0ef6bd527b9b23b9ceef70dc8b4cd1ee83ca14541964e764ad23f5151204f0f",
			public_key: "09d704feec7161952a952f306cd96023810c6788478a1c9fc50e7281ab7893ac01",
			Address:    "deto1qyyawp87a3ckr9f2j5hnqmxevq3czrr83prc58ylc5889qdt0zf6cqg26e27g",
		},

		{
			name:       "Deutsch",
			seed:       "Dekade Spagat Bereich Radclub Yeti Dialekt Unimog Nomade Anlage Hirte Besitz Märzluft Krabbe Nabel Halsader Chefarzt Hering tauchen Neuerung Reifen Umgang Hürde Alchimie Amnesie Reifen",
			secret_key: "a00b3c431e0037426f12b255aaca918863c8bbc690ff3765564bcc1de7fbb303",
			public_key: "2fe01ee7d22dca4a3e941920a96e1d901b1ec27d9c1baefbaf3520f8449c311400",
			Address:    "deto1qyh7q8h86gku5j37jsvjp2twrkgpk8kz0kwphthm4u6jp7zynsc3gqq6l75cr",
		},

		{
			name:       "Español",
			seed:       "perfil lujo faja puma favor pedir detalle doble carbón neón paella cuarto ánimo cuento conga correr dental moneda león donar entero logro realidad acceso doble",
			secret_key: "4f1101c4cc6adc6e6a63acde4e71fd76dc4471fa54769866d5e80a0a3d53d00c",
			public_key: "13d654f97f00eec1f4360ddd0f510bc2068de902acabafd99b62c2e733fad1f401",
			Address:    "deto1qyfav48e0uqwas05xcxa6r63p0pqdr0fq2k2ht7end3v9eenltglgqgeytm4d",
		},

		{
			name:       "Français",
			seed:       "lisser onctueux pierre trace flair riche machine ordre soir nougat talon balle biceps crier trame tenu gorge cuisine taverne presque laque argent roche secte ordre",
			secret_key: "340ed50ef73ba172e2fef23dc9f60e314f609bb7692d22d59f3938c391570b0a",
			public_key: "0d64b483b1b8b8ae8322cb222dabdd3e6b298dc0d7cbeedba9151f2fad7f9c2d00",
			Address:    "deto1qyxkfdyrkxut3t5ryt9jytdtm5lxk2vdcrtuhmkm4y237tad07wz6qq9nsslt",
		},

		{
			name:       "Italiano",
			seed:       "sospiro uomo sommario orecchio muscolo testa avido sponda mutande levare lamento frumento volpe zainetto ammirare stufa convegno patente salto venire pianeta marinaio minuto moneta moneta",
			secret_key: "21cb90e631866952954cdcba042a3ae90407c40c052cc067226df0b454933502",
			public_key: "20b38fd222e1d445533f08ccb101dac8af89fcd3a53197dec805f487961ba31401",
			Address:    "deto1qyst8r7jytsag32n8uyvevgpmty2lz0u6wjnr977eqzlfpukrw33gqgakzhln",
		},

		{
			name:       "Nederlands",
			seed:       "veto hobo tolvrij cricket somber omaans lourdes kokhals ionisch lipman freon neptunus zimmerman rijbaan wisgerhof oudachtig nerd walraven ruis gevecht foolen onheilig rugnummer russchen cricket",
			secret_key: "921cbd7640df5fd12effb8f4269c5a47bac0ef3f75a0c25aa9f174f589801102",
			public_key: "2488d34361c3f7e5b59dc6ad2e4a39218186771f3364c5c8bd5dce5f34b86a5500",
			Address:    "deto1qyjg356rv8pl0ed4nhr26tj28yscrpnhruekf3wgh4wuuhe5hp492qqgx73kd",
		},

		{
			name:       "Português",
			seed:       "guloso caatinga enunciar newtoniano aprumo ilogismo vazio gibi imovel mixuruca bauxita paludismo unanimidade zumbi vozes roer anzol leonardo roer ucraniano elmo paete susto taco imovel",
			secret_key: "92da2722c7138e65559973131fd2c69b4a0ae4faf0e12b7abe096d5870d6a700",
			public_key: "2559f252768176bd372ccd5a8096db38fa90a5bb98b998324ff01f6a801baffd00",
			Address:    "deto1qyj4nujjw6qhd0fh9nx44qykmvu04y99hwvtnxpjflcp765qrwhl6qqjlphm7",
		},

		{
			name:       "русский язык",
			seed:       "шорох рента увлекать пешеход гонка сеять пчела ваза апатия пишущий готовый вибрация юбка здоровье машина штука охрана доза рынок клоун рецепт отпуск шестерка эволюция вибрация",
			secret_key: "3d0fb729be695b865c073eed68ee91f06d429a27f8eaaaa6a99f954edbef8406",
			public_key: "05b41369bf3c8f19f5fb845d015760874d17a4e67f9eeb52607c644a1fb12e4501",
			Address:    "deto1qyzmgymfhu7g7x04lwz96q2hvzr569ayuelea66jvp7xgjslkyhy2qg6q6ujr",
		},

		{
			name:       "日本語",
			seed:       "かわく ねまき けもの せいげん ためる にんめい てあみ にりんしゃ さわやか えらい うちき けいかく あたる せっきゃく ずっしり かいよう おおや てらす くれる ばかり なこうど たいうん そまつ たいえき せいげん",
			secret_key: "e12da07065554a32ac798396f74dbb35557164f9f39291a3f95705e62e0d7703",
			public_key: "2afb7287eacb584d7771b1df7b6626eb994b4cbf5f30ca63bfbc216ec14930b401",
			Address:    "dero1qy40ku58at94snthwxca77mxym4ejj6vha0npjnrh77zzmkpfyctgqgry2zcv",
		},

		{
			name:       "简体中文 (中国)",
			seed:       "启 写 输 苯 加 担 乳 代 集 预 懂 均 倒 革 熟 载 隆 台 幸 谋 轮 抚 扩 急 输",
			secret_key: "3bfd99190f28bd7830b3631cfa514176fc24e88281fe056ce447a5a7fcdc9a02",
			public_key: "07c8cb5425af4b9956392f3e8c1610224527b361d61d55eba61f400f5d6c2bf000",
			Address:    "dero1qyru3j65ykh5hx2k8yhnarqkzq3y2fanv8tp640t5c05qr6ads4lqqqmtg4vg",
		},

		{
			name:       "Esperanto",
			seed:       "amrakonto facila vibri obtuza gondolo membro alkoholo oferti ciumi reinspekti azteka kupro gombo keglo dugongo diino hemisfero sume servilo bambuo sekretario alta diurno duloka hemisfero",
			secret_key: "61abd2a5625a95371882117d8652e0735779b7b535008c73d65735b9477b1105",
			public_key: "13d45cddec136b9f9842915b1bab62bfc77cddade156f2fa386316caf92f43a801",
			Address:    "dero1qyfaghxaasfkh8ucg2g4kxatv2luwlxa4hs4duh68p33djhe9ap6sqgt35auv",
		},
	}

	for _, test := range tests {

		account, err := Generate_Account_From_Recovery_Words(test.seed)
		if err != nil || account.SeedLanguage != test.name {
			t.Fatalf("%s Mnemonics testing failed err %s", test.name, err)
		}

		// test secret_key and public

		if test.secret_key != fmt.Sprintf("%s", account.Keys.Secret) {
			t.Fatalf("%s Wallet testing failed  expected secret key %s  actual %x", test.name, test.secret_key, account.Keys.Secret)

		}

		if test.public_key != fmt.Sprintf("%x", account.Keys.Public) {
			t.Fatalf("%s Wallet testing failed  expected public key %s  actual %x", test.name, test.public_key, account.Keys.Public)

		}

		// check addrees only if mainnet address
		if strings.Contains(test.Address, "deto") {
			if account.GetAddress().String() != test.Address {
				t.Fatalf("%s Wallet testing failed  expected address %s  actual %s", test.name, test.Address,
					account.GetAddress().String())
			}
		}

		// test checksum failure
		test := tests[0]
		seed := "sequence atlas unveil summon pebbles tuesday beer rudely snake rockets different fuselage woven tagged bested dented vegan hover rapid fawns obvious muppet randomly seasons seasons"
		account, err = Generate_Account_From_Recovery_Words(seed)
		if err == nil {
			t.Errorf("%s Account recovery failed err %s", test.name, err)
		}

	}

}
